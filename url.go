package letterboxd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/apex/log"
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
			log.WithFields(log.Fields{
				"path":       path,
				"profession": profession,
				"actor":      actor,
			}).Debug("Detected filmography")
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
		log.WithFields(log.Fields{
			"path": path,
			"user": user,
		}).Debug("Detected watchlist")
		items, _, err := u.client.User.WatchList(context.Background(), user)
		log.Info("Got items from /watchlist")
		if err != nil {
			return nil, err
		}
		return items, nil
	}

	// Handle user lists here
	if strings.Contains(path, "/list/") {
		user := strings.Split(path, "/")[1]
		list := strings.Split(path, "/")[3]
		log.WithFields(log.Fields{
			"path": path,
			"user": user,
			"list": list,
		}).Info("Detected user list")
		filmC := make(chan *Film)
		errorC := make(chan error)
		go u.client.User.StreamListWithChan(nil, user, list, filmC, errorC)
		items, err := slurpFilms(filmC, errorC)
		if err != nil {
			return nil, err
		}
		return items, nil
	}
	if strings.HasSuffix(path, "/films") {
		user := strings.Split(path, "/")[1]
		log.WithFields(log.Fields{
			"path": path,
			"user": user,
		}).Debug("Detected user films")
		items, _, err := u.client.User.Watched(nil, user)
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
		log.WithError(err).Debug("Error parsing URL")
		return "", err
	}
	if !strings.Contains(u.Hostname(), "letterboxd.com") {
		return "", errors.New("not a letterboxd URL")
	}
	return u.Path, nil
}
