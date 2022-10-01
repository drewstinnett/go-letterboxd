package letterboxd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
)

type UserService interface {
	Exists(context.Context, string) (bool, error)
	Profile(context.Context, string) (*User, *Response, error)
	StreamList(context.Context, string, string, chan *Film, chan error)
	StreamWatched(context.Context, string, chan *Film, chan error)
	StreamWatchList(context.Context, string, chan *Film, chan error)
	Watched(context.Context, string) ([]*Film, *Response, error)
	WatchList(context.Context, string) ([]*Film, *Response, error)
}

type User struct {
	Username         string `json:"username"`
	Bio              string `json:"bio,omitempty"`
	WatchedFilmCount int    `json:"watched_film_count"`
}

type UserServiceOp struct {
	client *Client
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

func (u *UserServiceOp) Profile(ctx context.Context, userID string) (*User, *Response, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", u.client.BaseURL, userID), nil)
	if err != nil {
		return nil, nil, err
	}
	user, resp, err := u.client.sendRequest(req, ExtractUser)
	if err != nil {
		return nil, resp, err
	}
	defer resp.Body.Close()
	return user.Data.(*User), resp, nil
}

func (u *UserServiceOp) Exists(ctx context.Context, userID string) (bool, error) {
	return false, nil
}

func (u *UserServiceOp) WatchList(ctx context.Context, userID string) ([]*Film, *Response, error) {
	log.Info().Msg("Starting WatchList sub")
	var previews []*Film
	page := 1
	// TODO: This can loop forever
	for {
		log.Info().Int("page", page).Msg("pagination")
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s/watchlist/page/%d", u.client.BaseURL, userID, page), nil)
		if err != nil {
			return nil, nil, err
		}
		// var previews []FilmPreview
		items, resp, err := u.client.sendRequest(req, ExtractUserFilms)
		if err != nil {
			return nil, resp, err
		}
		partialFilms := items.Data.([]*Film)
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
		var lastFilms []*Film
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

func (u *UserServiceOp) Watched(ctx context.Context, userID string) ([]*Film, *Response, error) {
	var previews []*Film
	// Get the first page. This sets the pagination.
	partialFirstFilms, pagination, err := u.client.Film.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("%s/%s/films/page/1", u.client.BaseURL, userID))
	if err != nil {
		return nil, nil, err
	}
	previews = append(previews, partialFirstFilms...)
	for i := 2; i <= pagination.TotalPages; i++ {
		partialFilms, _, err := u.client.Film.ExtractEnhancedFilmsWithPath(ctx, fmt.Sprintf("%s/%s/films/page/%v/", u.client.BaseURL, userID, i))
		if err != nil {
			log.Warn().Int("page", i).Str("user", userID).Msg("Failed to extract films")
			return nil, nil, err
		}
		previews = append(previews, partialFilms...)
	}

	return previews, nil, nil
}

func ExtractUserFilms(r io.Reader) (interface{}, *Pagination, error) {
	var previews []*Film
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
		var lastFilms []*Film
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
		var lastFilms []*Film
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
