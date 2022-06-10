package letterboxd

import (
	"errors"
	"strings"
)

// 0 means undefined
// -1 means go for as far as you can!
func normalizeStartStop(firstPage, lastPage int) (int, int, error) {
	if firstPage == 0 && lastPage == 0 {
		return 1, 1, nil
	} else if firstPage == 0 {
		return 1, lastPage, nil
	} else if lastPage == 0 {
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

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// Given a slice of strings, return a slice of ListIDs
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
