package letterboxd

import (
	"context"
)

type ListService interface {
	// ListFilms(context.Context, *ListFilmsOpt) ([]*Film, error)
	GetOfficialMap(context.Context) map[string]string
	GetOfficial(context.Context) []*ListID
}

type ListServiceOp struct {
	client *Client
}

type ListID struct {
	User string
	Slug string
}

// ListFilmsOpt is the options for the ListFilms method
type ListFilmsOpt struct {
	User      string // Username of the user for the list. Example: 'dave'
	Slug      string // Slug of the list: Example: 'official-top-250-narrative-feature-films'
	FirstPage int    // First page to fetch. Defaults to 1
	LastPage  int    // Last page to fetch. Defaults to FirstPage. Use -1 to fetch all pages
}

func (l *ListServiceOp) GetOfficialMap(ctx context.Context) map[string]string {
	ret := map[string]string{}
	for _, i := range l.GetOfficial(ctx) {
		ret[i.Slug] = i.User
	}
	return ret
}

func (l *ListServiceOp) GetOfficial(ctx context.Context) []*ListID {
	return []*ListID{
		{User: "crew", Slug: "edgar-wrights-1000-favorite-movies"},
		{User: "darrencb", Slug: "letterboxds-top-250-horror-films"},
		{User: "dave", Slug: "official-top-250-narrative-feature-films"},
		{User: "dave", Slug: "imdb-top-250"},
		{User: "gubarenko", Slug: "1001-movies-you-must-see-before-you-die-2021"},
		{User: "jack", Slug: "official-top-250-documentary-films"},
		{User: "jack", Slug: "women-directors-the-official-top-250-narrative"},
		{User: "jake_ziegler", Slug: "academy-award-winners-for-best-picture"},
		{User: "lifeasfiction", Slug: "letterboxd-100-animation/"},
		{User: "liveandrew", Slug: "bfi-2012-critics-top-250-films"},
		{User: "matthew", Slug: "box-office-mojo-all-time-worldwide"},
		{User: "moseschan", Slug: "afi-100-years-100-movies"},
	}
}
