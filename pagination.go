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

// Pagination contains all the information about a pages pagination
type Pagination struct {
	CurrentPage  int  `json:"current_page"`
	NextPage     int  `json:"next_page"`
	TotalPages   int  `json:"total_pages"`
	TotalItems   int  `json:"total_items"`
	ItemsPerPage int  `json:"items_per_page"`
	IsLast       bool `json:"is_last"`
}

// SetTotalItems will set the TotalItems count, along with anything else that needs an update
func (p *Pagination) SetTotalItems(i int) {
	p.TotalItems = i
	if p.ItemsPerPage != 0 {
		p.TotalPages = (p.TotalItems / p.ItemsPerPage) + 1
	}
}

func (p *Pagination) parseDivPaginationNext(s *goquery.Selection) {
	nlink := s.Find("a.next").First()
	if nlink.Text() == "Next" {
		href, ok := nlink.Attr("href")
		if !ok {
			return
		}
		parts := strings.Split(href, "/")
		next, err := strconv.Atoi(parts[len(parts)-2])
		if err != nil {
			return
		}
		p.CurrentPage = next - 1
	}
}

func (p *Pagination) parseDivPaginationPrevious(s *goquery.Selection) {
	plink := s.Find("a.previous").First()
	if plink.Text() == "Previous" {
		href, ok := plink.Attr("href")
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
}

func (p *Pagination) parseDivPagination(doc *goquery.Document) {
	doc.Find("div.pagination").Each(func(i int, s *goquery.Selection) {
		p.parseDivPaginationNext(s)
		p.parseDivPaginationPrevious(s)
	})
}

func paginationFromDivPaginatePages(doc *goquery.Document) (*Pagination, error) {
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
	return paginationIfCurrent(p)
}

func paginationFromBlockHeading(doc *goquery.Document) (*Pagination, error) {
	p := &Pagination{
		ItemsPerPage: 72,
	}
	doc.Find("p.ui-block-heading").Each(func(i int, s *goquery.Selection) {
		matches := regexp.MustCompile(`There are (\d+)`).FindStringSubmatch(strings.ReplaceAll(strings.TrimSpace(s.Text()), ",", ""))
		if len(matches) > 1 {
			count, err := strconv.Atoi(matches[1])
			if err != nil {
				return
			}
			p.SetTotalItems(count)
			p.parseDivPagination(doc)
		}
	})
	return paginationIfCurrent(p)
}

func paginationIfCurrent(p *Pagination) (*Pagination, error) {
	if p.CurrentPage == 0 {
		return p, errors.New("no pagination found")
	}
	return p, nil
}

// ExtractPaginationWithDoc returns a pagination object from a goquery Doc
func ExtractPaginationWithDoc(doc *goquery.Document) (*Pagination, error) {
	p, err := paginationFromDivPaginatePages(doc)
	// Hmmm, haven't found pagination info yet, check to see if it's one of those weird film list pages
	if err != nil {
		p, err = paginationFromBlockHeading(doc)
	}

	// Dang, still no pagination??
	if err != nil {
		return nil, errors.New("could not extract pagination, no current page")
	}
	if p.CurrentPage == p.TotalPages {
		p.IsLast = true
	} else {
		p.NextPage = p.CurrentPage + 1
	}
	return p, nil
}

// ExtractPaginationWithBytes pulls the pagination in from a given byte array
func ExtractPaginationWithBytes(b []byte) (*Pagination, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return ExtractPaginationWithDoc(doc)
}

// ExtractPaginationWithReader pulls the pagination from an io.Reader
func ExtractPaginationWithReader(r io.Reader) (*Pagination, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	return ExtractPaginationWithDoc(doc)
}

func extractHasNext(r io.Reader) bool {
	doc, err := goquery.NewDocumentFromReader(r)
	panicIfErr(err)

	var ret bool
	doc.Find("div.pagination").Find("a.next").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if s.Text() == "Next" {
			ret = true
		}
		return false
	})
	return ret
}

func extractHasNextWithBytes(r []byte) bool {
	return extractHasNext(bytes.NewReader(r))
}
