package letterboxd

import (
	"context"
)

type ListService interface {
	ListFilms(context.Context, *ListFilmsOpt) ([]*Film, error)
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
