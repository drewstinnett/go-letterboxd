package letterboxd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

// URLService is an interface for defining methods on a URL
type URLService interface {
	Items(ctx context.Context, url string) (interface{}, error)
}

// URLServiceOp is the operator for an URLService
type URLServiceOp struct {
	client *Client
}

// Items returns items from an URLService
func (u *URLServiceOp) Items(ctx context.Context, lurl string) (interface{}, error) {
	path, err := normalizeURLPath(lurl)
	if err != nil {
		return nil, err
	}
	// Check if this is a filmography first
	for _, profession := range Professions {
		if strings.HasPrefix(path, fmt.Sprintf("/%v/", profession)) {
			person := strings.Split(path, "/")[2]
			items, err := u.client.Film.Filmography(ctx, &FilmographyOpt{
				Profession: profession,
				Person:     person,
			})
			if err != nil {
				return nil, err
			}
			return items, nil
		}
	}
	// Handle Watchlist
	if strings.Contains(path, "/watchlist") {
		user := strings.Split(path, "/")[1]
		items, _, err := u.client.User.WatchList(context.TODO(), user)
		if err != nil {
			return nil, err
		}
		return items, nil
	}

	// Handle user lists here
	if strings.Contains(path, "/list/") {
		user := strings.Split(path, "/")[1]
		list := strings.Split(path, "/")[3]
		filmC := make(chan *Film)
		errorC := make(chan error)
		go u.client.User.StreamList(ctx, user, list, filmC, errorC)
		items, err := SlurpFilms(filmC, errorC)
		if err != nil {
			return nil, err
		}
		return items, nil
	}
	if strings.HasSuffix(path, "/films") {
		user := strings.Split(path, "/")[1]
		watchedC := make(chan *Film)
		doneC := make(chan error)
		go u.client.User.StreamWatched(ctx, user, watchedC, doneC)
		items, err := SlurpFilms(watchedC, doneC)
		if err != nil {
			return nil, err
		}
		return items, nil
	}

	// Default fail
	return nil, errors.New("could not find a match for that URL")
}

func normalizeURLPath(ourl string) (string, error) {
	ourl = strings.TrimSuffix(ourl, "/")
	if strings.HasPrefix(ourl, "/") {
		return ourl, nil
	}
	u, err := url.Parse(ourl)
	if err != nil {
		log.Debug().Err(err).Msg("Error parsing URL")
		return "", err
	}
	if !strings.Contains(u.Hostname(), "letterboxd.com") {
		return "", errors.New("not a letterboxd URL")
	}
	return u.Path, nil
}
