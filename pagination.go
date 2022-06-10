package letterboxd

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/apex/log"
)

type Pagination struct {
	CurrentPage int  `json:"current_page"`
	NextPage    int  `json:"next_page"`
	TotalPages  int  `json:"total_pages"`
	TotalItems  int  `json:"total_items"`
	IsLast      bool `json:"is_last"`
}

func ExtractPaginationWithDoc(doc *goquery.Document) (*Pagination, error) {
	p := &Pagination{}
	doc.Find("div.paginate-pages").Each(func(i int, s *goquery.Selection) {
		s.Find("li").Each(func(i int, s *goquery.Selection) {
			var err error
			if s.HasClass("paginate-current") {
				t := strings.TrimSpace(s.Text())
				if t != "…" {
					p.CurrentPage, err = strconv.Atoi(t)
					if err != nil {
						log.WithError(err).Debug("Error converting current page to int")
					}
					// Set current page to last, it should be overridden later
					p.TotalPages = p.CurrentPage
				}
			} else if s.HasClass("paginate-page") {
				t := strings.TrimSpace(s.Text())
				if t != "…" {
					p.TotalPages, err = strconv.Atoi(t)
					if err != nil {
						log.WithError(err).Debug("Error converting total page to int")
					}
				}
			}
		})
	})
	if p.CurrentPage == 0 {
		return nil, errors.New("Could not extract pagination, no current page")
	}
	if p.CurrentPage == p.TotalPages {
		p.IsLast = true
	} else {
		p.NextPage = p.CurrentPage + 1
	}
	return p, nil
}

func ExtractPaginationWithBytes(b []byte) (*Pagination, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return ExtractPaginationWithDoc(doc)
}

func ExtractPaginationWithReader(r io.Reader) (*Pagination, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	return ExtractPaginationWithDoc(doc)
}
