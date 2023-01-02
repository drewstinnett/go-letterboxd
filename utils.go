package letterboxd

import (
	"errors"
	"io"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// 0 means undefined
// -1 means go for as far as you can!
func normalizeStartStop(firstPage, lastPage int) (int, int, error) {
	switch {
	case firstPage == 0 && lastPage == 0:
		return 1, 1, nil
	case firstPage == 0:
		return 1, lastPage, nil
	case lastPage == 0:
		return firstPage, firstPage, nil
	}

	if (lastPage >= 0) && (firstPage > lastPage) {
		return 0, 0, errors.New("last page must be greater than first page")
	}

	return firstPage, lastPage, nil
}

func normalizeSlug(slug string) string {
	slug = strings.TrimPrefix(slug, "/film/")
	slug = strings.TrimSuffix(slug, "/")
	return slug
}

// stringInSlice is a tiny helper to determin if a slice of strings contains a specific string
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// ParseListArgs Given a slice of strings, return a slice of ListIDs
func ParseListArgs(args []string) ([]*ListID, error) {
	var ret []*ListID
	for _, argS := range args {
		if !strings.Contains(argS, "/") {
			return nil, errors.New("List Arg must contain a '/' (Example: username/list-slug)")
		}
		parts := strings.Split(argS, "/")
		lid := &ListID{
			User: parts[0],
			Slug: parts[1],
		}
		ret = append(ret, lid)
	}
	return ret, nil
}

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

func populateRemainingPages(count, total int, shuffle bool) []int {
	var remainingPages []int
	if shuffle {
		rand.Seed(time.Now().UnixNano())
		for i := 0; i <= count; i++ {
			// We don't care so much about the security of this random number
			remainingPages = append(remainingPages, rand.Intn(total-2+1)+2) // nolint:golint,gosec
		}
	} else {
		remainingPages = makeRange(2, count+1)
	}
	return remainingPages
}

func mustNewDocumentFromReader(r io.Reader) *goquery.Document {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		panic(err)
	}
	return doc
}

func mustParseURL(u string) *url.URL {
	u = strings.TrimSuffix(u, "/")
	url, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	return url
}

func min(values ...int) (min int) {
	if len(values) == 0 {
		panic("cannot detect a minimum value in an empty slice")
	}

	min = values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
	}

	return min
}

func max(values ...int) (max int) {
	if len(values) == 0 {
		panic("cannot detect a maximum value in an empty slice")
	}

	max = values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}

	return max
}

// stringOr returns a string, given a string and a default. Returns the default if the string is empty
func stringOr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
