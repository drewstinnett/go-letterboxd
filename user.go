package letterboxd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
)

type UserService interface {
	Exists(context.Context, string) (bool, error)
	Profile(context.Context, string) (*User, *Response, error)
	Following(context.Context, string) ([]string, *Response, error)
	Followers(context.Context, string) ([]string, *Response, error)
	// Interact with Diary
	StreamDiary(context.Context, string, chan *DiaryEntry, chan error)
	Diary(context.Context, string) (DiaryEntries, error)
	MustDiary(context.Context, string) DiaryEntries

	StreamList(context.Context, string, string, chan *Film, chan error)
	StreamWatched(context.Context, string, chan *Film, chan error)
	StreamWatchList(context.Context, string, chan *Film, chan error)
	WatchList(context.Context, string) (FilmSet, *Response, error)
	ExtractDiaryEntries(io.Reader) (interface{}, *Pagination, error)
}

type User struct {
	Username         string   `json:"username"`
	Bio              string   `json:"bio,omitempty"`
	WatchedFilmCount int      `json:"watched_film_count"`
	Following        []string `json:"following"`
	Followers        []string `json:"followers"`
}

type UserServiceOp struct {
	client *Client
}

func ExtractPeopleWithBytes(b []byte) (interface{}, *Pagination, error) {
	return ExtractPeople(bytes.NewReader(b))
}

func ExtractPeople(r io.Reader) (interface{}, *Pagination, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	hasNext := extractHasNextWithBytes(body)
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	p := &Pagination{
		IsLast: !hasNext,
	}
	ret := []string{}
	doc.Find("td.table-person").Find("a.name").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSuffix(strings.TrimPrefix(s.AttrOr("href", ""), "/"), "/")
		if name != "" {
			ret = append(ret, name)
		}
	})
	return ret, p, nil
}

func ExtractUser(r io.Reader) (interface{}, *Pagination, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, nil, err
	}
	user := &User{}
	doc.Find("section#person-bio").Each(func(i int, s *goquery.Selection) {
		s.Find("div.collapsible-text").Each(func(i int, s *goquery.Selection) {
			user.Bio = strings.TrimSpace(s.Text())
		})
	})
	doc.Find("section.js-profile-header").Each(func(i int, s *goquery.Selection) {
		user.Username = s.AttrOr("data-person", "")
	})
	doc.Find("div.profile-stats").Each(func(i int, s *goquery.Selection) {
		s.Find("a").Each(func(i int, s *goquery.Selection) {
			if s.AttrOr("href", "") == fmt.Sprintf("/%v/films/", user.Username) {
				s.Find("span.value").Each(func(i int, s *goquery.Selection) {
					countS := strings.TrimSpace(s.Text())
					countS = strings.Replace(countS, ",", "", -1)
					count, err := strconv.Atoi(countS)
					if err != nil {
						log.Warn().Err(err).Msg("Failed to parse film count")
					}
					user.WatchedFilmCount = count
				})
				// user.WatchedFilmCount, _ = s.AttrOr("data-count", "").Atoi()
			}
		})
	})
	if user.Username == "" {
		return nil, nil, fmt.Errorf("Failed to extract user")
	}
	return user, nil, nil
}

// MustGetDiary See GetDiary, but will panic instead of returning an error
func (u *UserServiceOp) MustDiary(ctx context.Context, username string) DiaryEntries {
	items, err := u.Diary(ctx, username)
	panicIfErr(err)
	return items
}

// GetDiary returns all diary entries for a given order, sorted by watched date,
// with the most recent watches first
func (u *UserServiceOp) Diary(ctx context.Context, username string) (DiaryEntries, error) {
	items := DiaryEntries{}
	c := make(chan *DiaryEntry)
	dc := make(chan error)
	go u.StreamDiary(ctx, username, c, dc)
	for loop := true; loop; {
		select {
		case d := <-c:
			items = append(items, *d)
		case err := <-dc:
			if err != nil {
				log.Error().Err(err).Msg("Failed to get watched films")
				dc <- err
			} else {
				log.Debug().Msg("Finished getting watched films")
				loop = false
			}
		}
	}
	// Sort entries
	sort.Slice(items, func(i, j int) bool {
		return items[i].Watched.After(*items[j].Watched)
	})
	return items, nil
}

func (u *UserServiceOp) XStreamDiary(ctx context.Context, username string, dec chan *DiaryEntry) {
	var err error
	var pagination *Pagination
	defer func() {
		log.Debug().Msg("Closing StreamWatched")
		defer close(dec)
	}()
	log.Debug().Msg("About to start streaming fims")

	// Get the first page. This seeds the pagination.
	firstEntries, pagination, err := u.extractDiaryEntryWithPath(ctx, username, 1)
	if err != nil {
		log.Warn().Msg("Error looking up first page")
		// done <- err
	}
	for _, i := range firstEntries {
		dec <- i
	}

	itemsPerFullPage := len(firstEntries)
	pagination.TotalItems = itemsPerFullPage

	// If more than 1 page, get the last page too, which will likely be a
	// partial batch of films
	if pagination.TotalPages > 1 {
		var lastEntries []*DiaryEntry
		lastEntries, _, err = u.extractDiaryEntryWithPath(ctx, username, pagination.TotalPages)
		if err != nil {
			log.Warn().Msg("Error looking up last page")
			// done <- err
		}
		pagination.TotalItems = pagination.TotalItems + len(lastEntries)
		for _, film := range lastEntries {
			dec <- film
		}
	}
	// Gather up the middle pages here
	if pagination.TotalPages > 2 {
		pagination.TotalItems = pagination.TotalItems + ((pagination.TotalPages - 2) * itemsPerFullPage)
		middlePageCount := pagination.TotalPages - 2
		wg := sync.WaitGroup{}
		wg.Add(middlePageCount)
		for i := 2; i < pagination.TotalPages; i++ {
			go func(i int) {
				defer wg.Done()
				pfilms, _, err := u.extractDiaryEntryWithPath(ctx, username, i)
				if err != nil {
					log.Warn().
						Int("page", i).
						Str("user", username).
						Msg("Failed to extract diary entries")
					return
				}
				for _, film := range pfilms {
					dec <- film
				}
			}(i)
		}
		wg.Wait()
	}
}

func (u *UserServiceOp) StreamDiary(ctx context.Context, username string, dec chan *DiaryEntry, done chan error) {
	var err error
	var pagination *Pagination
	defer func() {
		log.Debug().Msg("Closing StreamWatched")
		done <- nil
	}()
	log.Debug().Msg("About to start streaming fims")

	// Get the first page. This seeds the pagination.
	firstEntries, pagination, err := u.extractDiaryEntryWithPath(ctx, username, 1)
	// firstEntries, pagination, err := u.client.User.extractDiaryEntryWithPath(ctx, fmt.Sprintf("%s/%s/films/page/1", u.client.BaseURL, userID))
	if err != nil {
		done <- err
	}
	for _, i := range firstEntries {
		dec <- i
	}

	itemsPerFullPage := len(firstEntries)
	pagination.TotalItems = itemsPerFullPage

	// If more than 1 page, get the last page too, which will likely be a
	// partial batch of films
	if pagination.TotalPages > 1 {
		var lastEntries []*DiaryEntry
		lastEntries, _, err = u.extractDiaryEntryWithPath(ctx, username, pagination.TotalPages)
		if err != nil {
			done <- err
		}
		pagination.TotalItems = pagination.TotalItems + len(lastEntries)
		for _, film := range lastEntries {
			dec <- film
		}
	}
	// Gather up the middle pages here
	if pagination.TotalPages > 2 {
		pagination.TotalItems = pagination.TotalItems + ((pagination.TotalPages - 2) * itemsPerFullPage)
		middlePageCount := pagination.TotalPages - 2
		wg := sync.WaitGroup{}
		wg.Add(middlePageCount)
		for i := 2; i < pagination.TotalPages; i++ {
			go func(i int) {
				defer wg.Done()
				pfilms, _, err := u.extractDiaryEntryWithPath(ctx, username, i)
				if err != nil {
					log.Warn().
						Int("page", i).
						Str("user", username).
						Msg("Failed to extract diary entries")
					return
				}
				for _, film := range pfilms {
					dec <- film
				}
			}(i)
		}
		wg.Wait()
	}
}

func (u *UserServiceOp) Profile(ctx context.Context, userID string) (*User, *Response, error) {
	req := MustNewRequest("GET", fmt.Sprintf("%s/%s", u.client.BaseURL, userID), nil)
	user, resp, err := u.client.sendRequest(req, ExtractUser)
	if err != nil {
		return nil, resp, err
	}
	defer resp.Body.Close()

	userD := user.Data.(*User)

	userD.Following, _, err = u.Following(ctx, userID)
	if err != nil {
		log.Warn().Str("user", userID).Msg("Could not get user following")
	}

	userD.Followers, _, err = u.Followers(ctx, userID)

	if err != nil {
		log.Warn().Str("user", userID).Msg("Could not get user followers")
	}

	return userD, resp, nil
}

/*
type stringsHasMore struct {
	Values  []string
	HasMore bool
}
*/

func (u *UserServiceOp) Followers(ctx context.Context, userID string) ([]string, *Response, error) {
	curP := 1
	allPeople := []string{}

	// TODO: Do we want a limit thing here?
	for {
		req := MustNewRequest("GET", fmt.Sprintf("%s/%s/followers/page/%v", u.client.BaseURL, userID, curP), nil)
		people, resp, err := u.client.sendRequest(req, ExtractPeople)
		if err != nil {
			return nil, resp, err
		}
		err = resp.Body.Close()
		if err != nil {
			return nil, resp, err
		}
		names := people.Data.([]string)
		allPeople = append(allPeople, names...)

		if resp.pagination.IsLast {
			break
		}
		curP++
	}
	return allPeople, nil, nil
}

func (u *UserServiceOp) Following(ctx context.Context, userID string) ([]string, *Response, error) {
	curP := 1
	allPeople := []string{}

	// TODO: Do we want a limit thing here?
	for {
		req := MustNewRequest("GET", fmt.Sprintf("%s/%s/following/page/%v", u.client.BaseURL, userID, curP), nil)
		people, resp, err := u.client.sendRequest(req, ExtractPeople)
		if err != nil {
			return nil, resp, err
		}
		err = resp.Body.Close()
		if err != nil {
			return nil, resp, err
		}
		names := people.Data.([]string)
		allPeople = append(allPeople, names...)

		// Future Drew, Check to see what hasMore is. Is it getting passed back right? I dunno...
		if resp.pagination.IsLast {
			break
		}
		curP++
	}
	return allPeople, nil, nil
}

func (u *UserServiceOp) Exists(ctx context.Context, userID string) (bool, error) {
	return false, nil
}

func (u *UserServiceOp) WatchList(ctx context.Context, userID string) (FilmSet, *Response, error) {
	log.Info().Msg("Starting WatchList sub")
	var previews FilmSet
	page := 1
	// TODO: This can loop forever
	for {
		log.Info().Int("page", page).Msg("pagination")
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s/watchlist/page/%d", u.client.BaseURL, userID, page), nil)
		if err != nil {
			return nil, nil, err
		}
		items, resp, err := u.client.sendRequest(req, ExtractUserFilms)
		if err != nil {
			return nil, resp, err
		}
		partialFilms := items.Data.(FilmSet)
		err = u.client.Film.EnhanceFilmList(ctx, &partialFilms)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to enhance film list")
		}
		previews = append(previews, partialFilms...)
		log.Debug().Interface("pagination", items.Pagintion).Msg("pagination")
		if items.Pagintion.IsLast {
			break
		}
		page++
	}
	return previews, nil, nil
}

func (u *UserServiceOp) StreamWatched(ctx context.Context, userID string, rchan chan *Film, done chan error) {
	var err error
	var pagination *Pagination
	defer func() {
		log.Debug().Msg("Closing StreamWatched")
		done <- nil
	}()
	log.Debug().Msg("About to start streaming fims")

	// Get the first page. This seeds the pagination.
	firstFilms, pagination, err := u.client.Film.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("%s/%s/films/page/1", u.client.BaseURL, userID))
	if err != nil {
		done <- err
	}
	for _, film := range firstFilms {
		rchan <- film
	}

	itemsPerFullPage := len(firstFilms)
	pagination.TotalItems = itemsPerFullPage

	// If more than 1 page, get the last page too, which will likely be a
	// partial batch of films
	if pagination.TotalPages > 1 {
		var lastFilms FilmSet
		lastFilms, _, err = u.client.Film.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("%s/%s/films/page/%v", u.client.BaseURL, userID, pagination.TotalPages))
		if err != nil {
			done <- err
		}
		pagination.TotalItems = pagination.TotalItems + len(lastFilms)
		for _, film := range lastFilms {
			rchan <- film
		}
	}
	// Gather up the middle pages here
	if pagination.TotalPages > 2 {
		pagination.TotalItems = pagination.TotalItems + ((pagination.TotalPages - 2) * itemsPerFullPage)
		middlePageCount := pagination.TotalPages - 2
		wg := sync.WaitGroup{}
		wg.Add(middlePageCount)
		for i := 2; i < pagination.TotalPages; i++ {
			go func(i int) {
				defer wg.Done()
				pfilms, _, err := u.client.Film.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("%s/%s/films/page/%v/", u.client.BaseURL, userID, i))
				if err != nil {
					log.Warn().
						Int("page", i).
						Str("user", userID).
						Msg("Failed to extract films")
					return
				}
				for _, film := range pfilms {
					rchan <- film
				}
			}(i)
		}
		wg.Wait()
	}
}

func ExtractUserFilms(r io.Reader) (interface{}, *Pagination, error) {
	var previews FilmSet
	var pageBuf bytes.Buffer
	tee := io.TeeReader(r, &pageBuf)
	doc, err := goquery.NewDocumentFromReader(tee)
	if err != nil {
		return nil, nil, err
	}
	doc.Find("li.poster-container").Each(func(i int, s *goquery.Selection) {
		s.Find("div").Each(func(i int, s *goquery.Selection) {
			if s.HasClass("film-poster") {
				f := Film{}
				f.ID = s.AttrOr("data-film-id", "")
				// f.Slug = s.AttrOr("data-film-slug", "")
				f.Slug = normalizeSlug(s.AttrOr("data-film-slug", ""))
				f.Target = s.AttrOr("data-target-link", "")
				// Real film name appears in the alt attribute for the poster
				s.Find("img.image").Each(func(i int, s *goquery.Selection) {
					f.Title = s.AttrOr("alt", "")
				})
				previews = append(previews, &f)
			}
		})
	})
	pagination, err := ExtractPaginationWithReader(&pageBuf)
	if err != nil {
		log.Warn().Msg("No pagination data found, assuming it to be a single page")
		pagination = &Pagination{
			CurrentPage: 1,
			NextPage:    1,
			TotalPages:  1,
			IsLast:      true,
		}
	}
	return previews, pagination, nil
}

func (u *UserServiceOp) StreamList(
	ctx context.Context,
	username string,
	slug string,
	rchan chan *Film,
	done chan error,
) {
	var err error
	var pagination *Pagination
	defer func() {
		log.Debug().Msg("Closing StreamList")
		done <- nil
	}()
	firstFilms, pagination, err := u.client.Film.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("%s/%s/list/%s/page/1", u.client.BaseURL, username, slug))
	if err != nil {
		done <- err
	}
	for _, film := range firstFilms {
		rchan <- film
	}

	itemsPerFullPage := len(firstFilms)
	pagination.TotalItems = itemsPerFullPage

	// If more than 1 page, get the last page too, which will likely be a
	// partial batch of films
	if pagination.TotalPages > 1 {
		var lastFilms FilmSet
		lastFilms, _, err = u.client.Film.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("%s/%s/list/%s/page/%v", u.client.BaseURL, username, slug, pagination.TotalPages))
		if err != nil {
			done <- err
		}
		pagination.TotalItems = pagination.TotalItems + len(lastFilms)
		for _, film := range lastFilms {
			rchan <- film
		}
	}
	// Gather up the middle pages here
	if pagination.TotalPages > 2 {
		pagination.TotalItems = pagination.TotalItems + ((pagination.TotalPages - 2) * itemsPerFullPage)
		middlePageCount := pagination.TotalPages - 2
		wg := sync.WaitGroup{}
		wg.Add(middlePageCount)
		for i := 2; i < pagination.TotalPages; i++ {
			go func(i int) {
				defer wg.Done()
				pfilms, _, err := u.client.Film.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("%s/%s/list/%v/page/%v/", u.client.BaseURL, username, slug, i))
				if err != nil {
					log.Warn().Int("page", i).Str("user", username).Msg("Failed to extract films")
					return
				}
				for _, film := range pfilms {
					rchan <- film
				}
			}(i)
		}
		wg.Wait()
	}
}

func (u *UserServiceOp) StreamWatchList(
	ctx context.Context,
	username string,
	rchan chan *Film,
	done chan error,
) {
	var err error
	var pagination *Pagination
	defer func() {
		log.Debug().Msg("Closing StreamWatchList")
		done <- nil
	}()
	firstFilms, pagination, err := u.client.Film.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("%s/%s/watchlist/page/1", u.client.BaseURL, username))
	if err != nil {
		done <- err
	}
	for _, film := range firstFilms {
		rchan <- film
	}

	itemsPerFullPage := len(firstFilms)
	pagination.TotalItems = itemsPerFullPage

	// If more than 1 page, get the last page too, which will likely be a
	// partial batch of films
	if pagination.TotalPages > 1 {
		var lastFilms FilmSet
		lastFilms, _, err = u.client.Film.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("%s/%s/watchlist/page/%v", u.client.BaseURL, username, pagination.TotalPages))
		if err != nil {
			done <- err
		}
		pagination.TotalItems = pagination.TotalItems + len(lastFilms)
		for _, film := range lastFilms {
			rchan <- film
		}
	}
	// Gather up the middle pages here
	if pagination.TotalPages > 2 {
		pagination.TotalItems = pagination.TotalItems + ((pagination.TotalPages - 2) * itemsPerFullPage)
		middlePageCount := pagination.TotalPages - 2
		wg := sync.WaitGroup{}
		wg.Add(middlePageCount)
		for i := 2; i < pagination.TotalPages; i++ {
			go func(i int) {
				defer wg.Done()
				pfilms, _, err := u.client.Film.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("%s/%s/watchlist/page/%v/", u.client.BaseURL, username, i))
				if err != nil {
					log.Warn().Int("page", i).Str("user", username).Msg("Failed to extract films")
					return
				}
				for _, film := range pfilms {
					rchan <- film
				}
			}(i)
		}
		wg.Wait()
	}
}

func (u *UserServiceOp) extractDiaryEntryWithPath(ctx context.Context, username string, page int) ([]*DiaryEntry, *Pagination, error) {
	var pData *PageData
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%v/films/diary/page/%v/", u.client.BaseURL, username, page), nil)
	if err != nil {
		return nil, nil, err
	}
	var resp *Response
	pData, resp, err = u.client.sendRequest(req, u.ExtractDiaryEntries)
	defer resp.Body.Close()
	if err != nil {
		return nil, nil, err
	}
	entries := pData.Data.([]*DiaryEntry)
	return entries, &pData.Pagintion, nil
}

func (u *UserServiceOp) ExtractDiaryEntries(r io.Reader) (interface{}, *Pagination, error) {
	entries := []*DiaryEntry{}
	_ = entries
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, nil, err
	}
	pagination, err := ExtractPaginationWithDoc(doc)
	if err != nil {
		return nil, nil, err
	}
	doc.Find(".diary-entry-edit").Each(func(i int, s *goquery.Selection) {
		entry := &DiaryEntry{}
		// Figure out date watched
		val, ok := s.Find("a").Attr("data-viewing-date")
		if !ok {
			log.Warn().Msg("Error finding viewing date")
		} else {
			t, err := time.Parse("2006-01-02", val)
			if err != nil {
				log.Warn().Err(err).Msg("Error parsing date")
			} else {
				entry.Watched = &t
			}
		}

		// Figure out if a date was specified
		sDateS, ok := s.Find("a").Attr("data-specified-date")
		if !ok {
			log.Warn().Msg("Error finding specified date")
		} else {
			if sDateS == "true" {
				entry.SpecifiedDate = true
			}
		}

		// Figure out if a date was a rewatch
		rewatchS, ok := s.Find("a").Attr("data-rewatch")
		if !ok {
			log.Warn().Msg("Error finding if this is a rewatch")
		} else {
			if rewatchS == "true" {
				entry.Rewatch = true
			}
		}

		// Figure out the rating
		val, ok = s.Find("a").Attr("data-rating")
		if !ok {
			log.Warn().Msg("Error finding rating")
		} else {
			rating, err := strconv.Atoi(val)
			if err != nil {
				log.Warn().Msg("Error getting rating")
			}
			entry.Rating = &rating
		}

		// Figure out the title slug
		val, ok = s.Find("a").Attr("data-film-poster")
		if !ok {
			log.Warn().Msg("Error finding movie slug")
		} else {
			parts := strings.Split(val, "/")
			if len(parts) != 5 {
				log.Warn().Interface("parts", parts).Msg("Hmmm...error converting film poster to slug")
			} else {
				entry.Slug = &parts[2]
			}
		}
		entry.Film, err = u.client.Film.Get(context.TODO(), *entry.Slug)
		if err != nil {
			log.Warn().Err(err).Msg("Error looking up film")
		}

		entries = append(entries, entry)
	})
	return entries, pagination, nil
}

func SlurpDiary(itemC chan *DiaryEntry, doneC chan error) ([]*DiaryEntry, error) {
	var ret []*DiaryEntry
	for loop := true; loop; {
		select {
		case film := <-itemC:
			ret = append(ret, film)
		case err := <-doneC:
			if err != nil {
				return nil, err
			}
			loop = false
		default:
		}
	}
	return ret, nil
}
