package letterboxd

import "time"

type DiaryEntry struct {
	Watched       *time.Time
	Rating        *int
	Rewatch       bool
	SpecifiedDate bool
	Film          *Film
	Slug          *string
}
