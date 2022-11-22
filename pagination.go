package letterboxd

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
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
						log.Debug().Err(err).Msg("Error converting current page to int")
					}
					// Set current page to last, it should be overridden later
					p.TotalPages = p.CurrentPage
				}
			} else if s.HasClass("paginate-page") {
				t := strings.TrimSpace(s.Text())
				if t != "…" {
					p.TotalPages, err = strconv.Atoi(t)
					if err != nil {
						log.Debug().Err(err).Msg("Error converting total page to int")
					}
				}
			}
		})
	})

	// Hmmm, haven't found pagination info yet, check to see if it's one of those weird film list pages
	if p.CurrentPage == 0 {
		doc.Find("p.ui-block-heading").Each(func(i int, s *goquery.Selection) {
			maybe := s.Text()
			maybe = strings.ReplaceAll(strings.TrimSpace(maybe), ",", "")
			r := regexp.MustCompile(`There are (\d+)`)
			matches := r.FindStringSubmatch(maybe)
			if len(matches) > 1 {
				count, err := strconv.Atoi(matches[1])
				if err != nil {
					log.Warn().Msg("Could not extract film count for pagination")
					return
				}
				p.TotalItems = count
				p.TotalPages = (count / 72) + 1
				// Ok, now try to detect the current page based on previous/next
				doc.Find("div.pagination").Each(func(i int, s *goquery.Selection) {
					nlink := s.Find("a.next").First()
					if nlink.Text() == "Next" {
						href, ok := nlink.Attr("href")
						if !ok {
							return
						}
						parts := strings.Split(href, "/")
						next, err := strconv.Atoi(parts[len(parts)-2])
						if err != nil {
							log.Warn().Err(err).Msg("Error detecting next page")
							return
						}
						p.CurrentPage = next - 1
					}
					plink := s.Find("a.previous").First()
					if plink.Text() == "Previous" {
						href, ok := nlink.Attr("href")
						if !ok {
							return
						}
						parts := strings.Split(href, "/")
						prev, err := strconv.Atoi(parts[len(parts)-2])
						if err != nil {
							log.Warn().Err(err).Msg("Error detecting previous page")
							return
						}
						p.CurrentPage = prev + 1
					}
				})
			}
		})
	}
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
