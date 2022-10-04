package letterboxd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

type URLService interface {
	Items(ctx context.Context, url string) (interface{}, error)
}

type URLServiceOp struct {
	client *Client
}

func (u *URLServiceOp) Items(ctx context.Context, lurl string) (interface{}, error) {
	path, err := normalizeURLPath(lurl)
	if err != nil {
		return nil, err
	}
	// Check if this is a filmography first
	professions := GetFilmographyProfessions()
	for _, profession := range professions {
		if strings.HasPrefix(path, fmt.Sprintf("/%v/", profession)) {
			actor := strings.Split(path, "/")[2]
			log.Debug().
				Str("path", path).
				Str("profession", profession).
				Str("actor", actor).
				Msg("Detected filmography")
			items, err := u.client.Film.Filmography(nil, &FilmographyOpt{
				Profession: profession,
				Person:     actor,
			})
			if err != nil {
				return nil, err
			}
			return items, nil

		}
	}
	// Handle Watchlist
	if strings.HasSuffix(path, "/watchlist") {
		user := strings.Split(path, "/")[1]
		log.Debug().
			Str("path", path).
			Str("user", user).
			Msg("Detected watchlist")
		items, _, err := u.client.User.WatchList(context.Background(), user)
		log.Info().Msg("Got items from /watchlist")
		if err != nil {
			return nil, err
		}
		return items, nil
	}

	// Handle user lists here
	if strings.Contains(path, "/list/") {
		user := strings.Split(path, "/")[1]
		list := strings.Split(path, "/")[3]
		log.Info().
			Str("path", path).
			Str("user", user).
			Str("list", list).
			Msg("Detected user list")
		filmC := make(chan *Film)
		errorC := make(chan error)
		go u.client.User.StreamList(nil, user, list, filmC, errorC)
		items, err := SlurpFilms(filmC, errorC)
		if err != nil {
			return nil, err
		}
		return items, nil
	}
	if strings.HasSuffix(path, "/films") {
		user := strings.Split(path, "/")[1]
		log.Debug().
			Str("path", path).
			Str("user", user).
			Msg("Detected user films")

		watchedC := make(chan *Film)
		doneC := make(chan error)
		go u.client.User.StreamWatched(nil, user, watchedC, doneC)
		items, err := SlurpFilms(watchedC, doneC)
		if err != nil {
			return nil, err
		}
		return items, nil
	}

	// Default fail
	return nil, errors.New("Could not find a match for that URL")
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
