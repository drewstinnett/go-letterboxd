package letterboxd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-redis/cache/v8"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
)

// ExternalFilmIDs references 3rd party IDs for a given film
type ExternalFilmIDs struct {
	IMDB string `json:"imdb"`
	TMDB string `json:"tmdb"`
}

// Film represents a Letterboxd Film
type Film struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Slug        string           `json:"slug"`
	Target      string           `json:"target"`
	Year        int              `json:"year"`
	ExternalIDs *ExternalFilmIDs `json:"external_ids,omitempty"`
}

// FilmService defines a service to handle methods against Letterboxd films
type FilmService interface {
	EnhanceFilm(context.Context, *Film) error
	EnhanceFilmList(context.Context, *FilmSet) error
	Filmography(context.Context, *FilmographyOpt) (FilmSet, error)
	Get(context.Context, string) (*Film, error)
	GetWatchedIMDBIDs(context.Context, string) ([]string, error)
	ExtractFilmsWithPath(context.Context, string) (FilmSet, *Pagination, error)
	ExtractEnhancedFilmsWithPath(context.Context, string) (FilmSet, *Pagination, error)
	StreamBatch(context.Context, *FilmBatchOpts, chan *Film, chan error)
	List(context.Context, *FilmListOpts) (FilmSet, error)
}

// FilmListOpts options for listing films
type FilmListOpts struct {
	SortBy       string
	ShufflePages bool
	PageCount    int
}

// FilmServiceOp is the operator for a FilmService
type FilmServiceOp struct {
	client *Client
}

// FilmographyOpt is the options for a filmography
type FilmographyOpt struct {
	Person     string // Person whos filmography is to be fetched
	Profession string // Profession of the person (actor, writer, director)
	// FirstPage  int    // First page to fetch. Defaults to 1
	// LastPage   int    // Last page to fetch. Defaults to FirstPage. Use -1 to fetch all pages
}

// List lists out all films using the given options
func (f *FilmServiceOp) List(ctx context.Context, opts *FilmListOpts) (FilmSet, error) {
	sortBy := opts.SortBy
	if sortBy == "" {
		sortBy = "popular"
	}
	pageCount := opts.PageCount
	if pageCount == 0 {
		pageCount = 1
	}

	// Always pull in the first page, so we can get the right pagination and whatnot
	allFilms, pagination, err := f.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("/films/ajax/%v/size/small/page/1", sortBy))
	if err != nil {
		return nil, err
	}

	if (pageCount > 1) && (pagination.TotalPages > 1) {
		remainingPages := populateRemainingPages(pageCount, pagination.TotalPages, opts.ShufflePages)
		for _, p := range remainingPages {
			films, _, err := f.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("/films/ajax/%v/size/small/page/%v", sortBy, p))
			if err != nil {
				return nil, err
			}
			allFilms = append(allFilms, films...)
		}
	}
	return allFilms, nil
}

// Validate ensures that filmography options contains the appropriate fields
func (f *FilmographyOpt) Validate() error {
	if f.Person == "" {
		return fmt.Errorf("person is required")
	}
	if f.Profession == "" {
		return fmt.Errorf("profession is required")
	}
	profs := GetFilmographyProfessions()
	if !StringInSlice(f.Profession, profs) {
		return fmt.Errorf("profession must be one of %v", profs)
	}
	return nil
}

// FilmBatchOpts provides options for retrieving a batch of films
type FilmBatchOpts struct {
	Watched   []string  `json:"watched"`
	List      []*ListID `json:"list"`
	WatchList []string  `json:"watchlist"`
}

// StreamBatch Get a bunch of different films at once and stream them back to the user
func (f *FilmServiceOp) StreamBatch(ctx context.Context, batchOpts *FilmBatchOpts, filmsC chan *Film, done chan error) {
	defer func() {
		log.Debug().Msg("Completed Stream Batch")
		done <- nil
	}()
	for _, username := range batchOpts.Watched {
		userFilmC := make(chan *Film)
		userDone := make(chan error)
		go f.client.User.StreamWatched(ctx, username, userFilmC, userDone)
		for loop := true; loop; {
			select {
			case film := <-userFilmC:
				filmsC <- film
			case err := <-userDone:
				if err != nil {
					done <- err
				}
				loop = false
			}
		}
	}
	for _, listID := range batchOpts.List {
		listFilmC := make(chan *Film)
		listDone := make(chan error)
		go f.client.User.StreamList(ctx, listID.User, listID.Slug, listFilmC, listDone)
		for loop := true; loop; {
			select {
			case film := <-listFilmC:
				filmsC <- film
			case err := <-listDone:
				if err != nil {
					done <- err
				}
				loop = false
			}
		}
	}

	for _, user := range batchOpts.WatchList {
		// userFilms := []Film{}
		listFilmC := make(chan *Film)
		listDone := make(chan error)
		go f.client.User.StreamWatchList(ctx, user, listFilmC, listDone)
		for loop := true; loop; {
			select {
			case film := <-listFilmC:
				filmsC <- film
			case err := <-listDone:
				if err != nil {
					done <- err
				}
				loop = false
			}
		}
	}
}

// ExtractFilmsWithPath Given a url path, return a list of films it contains
func (f *FilmServiceOp) ExtractFilmsWithPath(ctx context.Context, path string) (FilmSet, *Pagination, error) {
	u := mustParseURL(path)
	key := fmt.Sprintf("/letterboxd/fullpage%s", u.Path)
	var inCache bool
	var pData *PageData
	var films FilmSet

	if ctx == nil {
		ctx = context.Background()
	}

	if f.client.Cache != nil {
		if err := f.client.Cache.Get(ctx, key, &pData); err == nil {
			inCache = true
			fi := pData.Data.([]interface{})
			for _, i := range fi {
				var d Film
				err := mapstructure.Decode(i, &d)
				if err != nil {
					return nil, nil, err
				}
				films = append(films, &d)
			}
		}
	}

	if !inCache {
		log.Debug().Str("key", key).Msg("Page not in cache, fetching from Letterboxd.com")
		var url string
		if strings.HasPrefix(path, "http") {
			url = path
		} else {
			url = fmt.Sprintf("%v%v", f.client.BaseURL, path)
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, nil, err
		}
		var resp *Response
		pData, resp, err = f.client.sendRequest(req, ExtractUserFilms)
		if err != nil {
			return nil, nil, err
		}
		defer dclose(resp.Body) // nolint:golint,bodyclose
		log.Debug().Str("key", key).Msg("Page fetched from Letterboxd.com")
		films = pData.Data.(FilmSet)
		if f.client.Cache != nil {
			if err := f.client.Cache.Set(&cache.Item{
				Ctx:   ctx,
				Key:   key,
				Value: pData,
				TTL:   time.Hour * 6,
			}); err != nil {
				log.Warn().Err(err).Msg("Error Writing Cache")
			}
		}
	}

	return films, &pData.Pagintion, nil
}

// ExtractEnhancedFilmsWithPath returns a list of data enriched films from a URL path
func (f *FilmServiceOp) ExtractEnhancedFilmsWithPath(ctx context.Context, path string) (FilmSet, *Pagination, error) {
	films, pagination, err := f.ExtractFilmsWithPath(ctx, path)
	if err != nil {
		return nil, pagination, err
	}

	log.Debug().Msg("Launching EnhanceFilmList")
	err = f.client.Film.EnhanceFilmList(ctx, &films)
	if err != nil {
		return nil, nil, err
	}

	return films, pagination, nil
}

// Get returns a single film from the slug
func (f *FilmServiceOp) Get(ctx context.Context, slug string) (*Film, error) {
	// Determine if we need to get the cached version or not
	key := fmt.Sprintf("/letterboxd/film/%s", slug)
	var retFilm Film
	var inCache bool
	if ctx == nil {
		ctx = context.Background()
	}
	if f.client.Cache != nil {
		log.Debug().Msg("Using cache for lookup")
		if err := f.client.Cache.Get(ctx, key, &retFilm); err == nil {
			log.Debug().Str("key", key).Msg("Found film in cache")
			inCache = true
		} else {
			log.Debug().Err(err).Str("key", key).Msg("Found NOT film in cache")
		}
	}

	if !inCache {
		log.Debug().Str("key", key).Msg("Film not in cache, fetching from Letterboxd.com")
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/film/%s", f.client.BaseURL, slug), nil)
		if err != nil {
			return nil, err
		}
		item, resp, err := f.client.sendRequest(req, extractFilmFromFilmPage)
		if err != nil {
			return nil, err
		}
		defer dclose(resp.Body)
		retFilm = *item.Data.(*Film)
		log.Debug().Str("key", key).Msg("Film fetched from Letterboxd.com")

		if f.client.Cache != nil {
			if err := f.client.Cache.Set(&cache.Item{
				Ctx:   ctx,
				Key:   key,
				Value: retFilm,
				TTL:   time.Hour * 24 * 7,
			}); err != nil {
				log.Warn().Err(err).Msg("Error Writing Cache")
			}
		}
	}
	return &retFilm, nil
}

// Filmography returns the Filmography based on certain options
func (f *FilmServiceOp) Filmography(ctx context.Context, opt *FilmographyOpt) (FilmSet, error) {
	var films FilmSet
	err := opt.Validate()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s/%s", f.client.BaseURL, opt.Profession, opt.Person), nil)
	if err != nil {
		return nil, err
	}
	items, resp, err := f.client.sendRequest(req, extractFilmography)
	if err != nil {
		return nil, err
	}
	defer dclose(resp.Body)

	partialFilms := items.Data.(FilmSet)

	// This is a bit costly, parallel time?
	err = f.client.Film.EnhanceFilmList(ctx, &partialFilms)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to enhance film list")
		return nil, err
	}

	films = append(films, partialFilms...)
	err = f.client.Film.EnhanceFilmList(ctx, &films)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to enhance film list")
		return nil, err
	}

	return films, nil
}

// EnhanceFilm Given a film, with some minimal information (like the slug), get as much data as you can
func (f *FilmServiceOp) EnhanceFilm(ctx context.Context, film *Film) error {
	if film.Slug == "" {
		return errors.New("Film has no slug. Needs that to enhance")
	}
	fullFilm, err := f.Get(ctx, film.Slug)
	if err != nil {
		return errors.New("failed to get film for enhancement")
	}
	if film.Year == 0 {
		film.Year = fullFilm.Year
	}
	if film.Title == "" {
		film.Title = fullFilm.Title
	}
	if film.ExternalIDs == nil {
		film.ExternalIDs = fullFilm.ExternalIDs
	}
	if film.ID == "" {
		film.ID = fullFilm.ID
	}
	return nil
}

// EnhanceFilmList takes a list of films, and returns the enhanced version
func (f *FilmServiceOp) EnhanceFilmList(ctx context.Context, films *FilmSet) error {
	var wg sync.WaitGroup
	wg.Add(len(*films))
	guard := make(chan struct{}, 5)
	for _, film := range *films {
		go func(film *Film) {
			defer wg.Done()
			guard <- struct{}{}
			log.Debug().Str("slug", film.Slug).Msg("Looking up film")
			if err := f.EnhanceFilm(ctx, film); err != nil {
				log.Warn().Err(err).Msg("Failed to get external IDs")
			}
			<-guard
		}(film)
	}
	wg.Wait()
	return nil
}

func extractFilmFromFilmPage(r io.Reader) (interface{}, *Pagination, error) {
	f := &Film{
		ExternalIDs: &ExternalFilmIDs{},
	}
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, nil, err
	}
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if val, ok := s.Attr("property"); ok && val == "og:title" {
			fullTitle := s.AttrOr("content", "")
			f.Year, err = extractYearFromTitle(fullTitle)
			if err != nil {
				log.Debug().Err(err).Str("fullTitle", fullTitle).Msg("Error detecting year")
			} else {
				f.Title = fullTitle[0 : len(fullTitle)-7]
			}
		}
	})
	doc.Find("div").Find("div").Each(func(i int, s *goquery.Selection) {
		if s.HasClass("poster film-poster") {
			if f.Slug == "" {
				f.Slug = normalizeSlug(s.AttrOr("data-film-slug", ""))
			}
			if f.Target == "" {
				f.Target = s.AttrOr("data-target-link", "")
			}
			if f.ID == "" {
				f.ID = s.AttrOr("data-film-id", "")
			}
		}
	})
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		if val, ok := s.Attr("data-track-action"); ok && val == "IMDb" {
			f.ExternalIDs.IMDB = extractIDFromURL(s.AttrOr("href", ""))
		}
		if val, ok := s.Attr("data-track-action"); ok && val == "TMDb" {
			f.ExternalIDs.TMDB = extractIDFromURL(s.AttrOr("href", ""))
		}
	})
	return f, nil, nil
}

func extractIDFromURL(url string) string {
	if strings.Contains(url, "imdb.com") {
		return strings.Split(url, "/")[4]
	} else if strings.Contains(url, "themoviedb.org") {
		return strings.Split(url, "/")[4]
	}
	return ""
}

func previewsWithDoc(doc *goquery.Document) FilmSet {
	var previews FilmSet
	doc.Find("li.poster-container").Each(func(i int, s *goquery.Selection) {
		s.Find("div").Each(func(i int, s *goquery.Selection) {
			if s.HasClass("film-poster") {
				f := Film{}
				f.ID = s.AttrOr("data-film-id", "")
				f.Slug = normalizeSlug(s.AttrOr("data-film-slug", ""))
				f.Target = s.AttrOr("data-target-link", "")
				s.Find("img.image").Each(func(i int, s *goquery.Selection) {
					f.Title = s.AttrOr("alt", "")
				})
				previews = append(previews, &f)
			}
		})
	})
	return previews
}

func extractFilmography(r io.Reader) (interface{}, *Pagination, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, nil, err
	}
	previews := previewsWithDoc(doc)
	return previews, nil, nil
}

// GetFilmographyProfessions is just a hard coded list of professions. Should this be a constant instead?
func GetFilmographyProfessions() []string {
	return []string{"actor", "director", "producer", "writer"}
}

// GetWatchedIMDBIDs returns a list of imdb ids that have been watched by a given user
func (f *FilmServiceOp) GetWatchedIMDBIDs(ctx context.Context, username string) ([]string, error) {
	wfilmC := make(chan *Film)
	wdoneC := make(chan error)

	go f.client.User.StreamWatched(ctx, username, wfilmC, wdoneC)

	var watchedIDs []string
	for loop := true; loop; {
		select {
		case film := <-wfilmC:
			if film.ExternalIDs != nil {
				watchedIDs = append(watchedIDs, film.ExternalIDs.IMDB)
			} else {
				log.Debug().Str("title", film.Title).Msg("No external IDs, skipping")
			}
		case err := <-wdoneC:
			if err != nil {
				log.Error().Err(err).Msg("Failed to get watched films")
				wdoneC <- err
			} else {
				log.Debug().Msg("Finished getting watched films")
				loop = false
			}
		}
	}
	return watchedIDs, nil
}

// SlurpFilms Helper blocking function to slurp a batch of films from the
// streaming calls. This negates the whole 'Streaming' thing, so use sparingly
func SlurpFilms(filmC chan *Film, errorC chan error) (FilmSet, error) {
	var ret FilmSet
	for loop := true; loop; {
		select {
		case film := <-filmC:
			ret = append(ret, film)
		case err := <-errorC:
			if err != nil {
				return nil, err
			}
			loop = false
		default:
		}
	}
	return ret, nil
}

func extractYearFromTitle(title string) (int, error) {
	var year int
	var err error
	if len(title) < 7 {
		return 0, errors.New("title is too short")
	}
	if !strings.Contains(title, "(") || !strings.Contains(title, ")") {
		return 0, errors.New("title does not contain parenthesis")
	}
	rawYear := title[len(title)-5 : len(title)-1]
	year, err = strconv.Atoi(rawYear)
	if err != nil {
		return 0, err
	}
	return year, nil
}

func makeRange(min, max int) []int {
	a := make([]int, max-min+1)
	for i := range a {
		a[i] = min + i
	}
	return a
}

// FilmSet is just a list of pointers to Film items
type FilmSet []*Film

// IMDBIDs returns a list of IMDB IDs from a FilmSet
func (fs *FilmSet) IMDBIDs() []string {
	ids := make([]string, len(*fs))
	for idx, item := range *fs {
		ids[idx] = item.ExternalIDs.IMDB
	}
	return ids
}

// TMDBIDs returns a list of TMDB IDs from a FilmSet
func (fs *FilmSet) TMDBIDs() []string {
	ids := make([]string, len(*fs))
	for idx, item := range *fs {
		ids[idx] = item.ExternalIDs.TMDB
	}
	return ids
}
