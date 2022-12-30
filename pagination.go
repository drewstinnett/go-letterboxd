package letterboxd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog/log"
)

// Paginationer is anything that can return Pagination data when given a goquery.Document
type paginationer func(*goquery.Document) (*Pagination, error)

// paginationers are all of the functions we have to detect pagination
var paginationers []paginationer = []paginationer{
	paginationFromDivPaginatePages,
	paginationFromBlockHeading,
}

// Pagination contains all the information about a pages pagination
type Pagination struct {
	CurrentPage  int  `json:"current_page"`
	NextPage     int  `json:"next_page"`
	TotalPages   int  `json:"total_pages"`
	TotalItems   int  `json:"total_items"`
	ItemsPerPage int  `json:"items_per_page"`
	IsLast       bool `json:"is_last"`
}

// complete fills in whatever available that is missing info is in the Pagination object
func (p *Pagination) complete() {
	if p.CurrentPage == p.TotalPages {
		p.IsLast = true
	} else {
		p.NextPage = p.CurrentPage + 1
	}
}

// SetTotalItems will set the TotalItems count, along with anything else that needs an update based on the TotalItems
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
		fmt.Fprintf(os.Stderr, "DIIIING")
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
		return nil, errors.New("no pagination found")
	}
	return p, nil
}

// ExtractPaginationWithDoc returns a pagination object from a goquery Doc
func ExtractPaginationWithDoc(doc *goquery.Document) (*Pagination, error) {
	var p *Pagination
	for _, pa := range paginationers {
		var err error
		p, err = pa(doc)
		if err == nil {
			break
		}
	}
	// Dang, still no pagination??
	if p == nil {
		return nil, errors.New("could not extract pagination, no current page")
	}
	p.complete()
	return p, nil
}

// ExtractPagination pulls the pagination from an io.Reader
func ExtractPagination(r io.Reader) (*Pagination, error) {
	doc := mustNewDocumentFromReader(r)
	return ExtractPaginationWithDoc(doc)
}

// hasNext returns true if a page has more pages to show.  This is needed for
// pagination that only shows if there is another page available, but not how
// many total pages there are
func hasNext(r io.Reader) bool {
	doc := mustNewDocumentFromReader(r)

	var ret bool
	doc.Find("div.pagination").Find("a.next").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if s.Text() == "Next" {
			ret = true
		}
		return false
	})
	return ret
}
