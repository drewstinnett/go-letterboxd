package letterboxd

import (
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
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

// Given a URL, return the page number it contains. This is usually the last dir in the path section
func pageWithURL(u string) (int, error) {
	url := mustParseURL(u)
	parts := strings.Split(url.Path, "/")
	page, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0, err
	}
	return page, nil
}

func (p *Pagination) parseDivPaginationNext(s *goquery.Selection) {
	nlink := s.Find("a.next").First()
	if nlink.Text() == "Next" {
		href, ok := nlink.Attr("href")
		if !ok {
			return
		}
		pageNo, err := pageWithURL(href)
		if err != nil {
			return
		}
		p.CurrentPage = pageNo - 1
		// Set next page if not already set
		if p.NextPage == 0 {
			p.NextPage = pageNo
		}
	}
}

func (p *Pagination) parseDivPaginationPrevious(s *goquery.Selection) {
	plink := s.Find("a.previous").First()
	if plink.Text() == "Previous" {
		href, ok := plink.Attr("href")
		if !ok {
			return
		}

		pageNo, err := pageWithURL(href)
		if err != nil {
			return
		}
		p.CurrentPage = pageNo + 1
	}
}

func (p *Pagination) parseDivPagination(doc *goquery.Document) {
	doc.Find("div.pagination").Each(func(i int, s *goquery.Selection) {
		p.parseDivPaginationNext(s)
		p.parseDivPaginationPrevious(s)
	})
}

func parsePaginateCurrent(s *goquery.Selection, p *Pagination) {
	if s.HasClass("paginate-current") {
		t := strings.TrimSpace(s.Text())
		if t != "…" {
			curP, err := strconv.Atoi(t)
			if err == nil {
				p.CurrentPage = curP
				// Set current page to last, it should be overridden later
				p.TotalPages = p.CurrentPage
			}
		}
	}
}

func parsePaginatePage(s *goquery.Selection, p *Pagination) {
	if s.HasClass("paginate-page") {
		t := strings.TrimSpace(s.Text())
		if t != "…" {
			totP, err := strconv.Atoi(t)
			if err == nil {
				p.TotalPages = totP
			}
		}
	}
}

func paginationFromDivPaginatePages(doc *goquery.Document) (*Pagination, error) {
	p := &Pagination{}
	doc.Find("div.paginate-pages").Find("li").Each(func(i int, s *goquery.Selection) {
		parsePaginateCurrent(s, p)
		parsePaginatePage(s, p)
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
			if err == nil {
				p.SetTotalItems(count)
				p.parseDivPagination(doc)
			}
		}
	})
	return paginationIfCurrent(p)
}

// paginationIfCurrent returns the pagination type _if_ a current page is set.
func paginationIfCurrent(p *Pagination) (*Pagination, error) {
	if p.CurrentPage == 0 {
		return nil, errors.New("no pagination found")
	}
	return p, nil
}

func paginationWithDoc(doc *goquery.Document) (*Pagination, error) {
	// Loop through all the pagination items we have, and return whichever
	// gives us pagination first
	var p *Pagination
	for _, pa := range paginationers {
		var err error
		p, err = pa(doc)
		if err == nil {
			p.complete()
			return p, nil
		}
	}
	return nil, errors.New("no pagination found")
}

// ExtractPaginationWithDoc returns a pagination object from a goquery Doc
func ExtractPaginationWithDoc(doc *goquery.Document) (*Pagination, error) {
	p, err := paginationWithDoc(doc)
	// Dang, still no pagination??
	if err != nil {
		return nil, errors.New("could not extract pagination, no current page")
	}
	return p, nil
}

// ExtractPagination pulls the pagination from an io.Reader
func ExtractPagination(r io.Reader) (*Pagination, error) {
	return ExtractPaginationWithDoc(mustNewDocumentFromReader(r))
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
