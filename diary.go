package letterboxd

import (
	"errors"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// DiaryEntry is a specific film from a users Diary
type DiaryEntry struct {
	Watched       *time.Time
	Rating        *int
	Rewatch       bool
	SpecifiedDate bool
	Film          *Film
	Slug          *string
}

// DiaryEntries is multiple DiaryEntry items
type DiaryEntries []*DiaryEntry

// DiaryFilterOpts provides options for filtering a user diary
type DiaryFilterOpts struct {
	Earliest      *time.Time
	Latest        *time.Time
	MinRating     *int
	MaxRating     *int
	Rewatch       *bool
	SpecifiedDate *bool
}

type (
	// DiaryFilter is a generic function to filter diary entries
	DiaryFilter func(DiaryEntry, DiaryFilterOpts) bool
	// DiaryFilterBulk is a generic function to filter diary entries in bulk
	DiaryFilterBulk func(DiaryEntries, DiaryFilterOpts) DiaryEntries
)

// DiaryFilterEarliest filters based on the earliest date
func DiaryFilterEarliest(e DiaryEntry, f DiaryFilterOpts) bool {
	if f.Earliest == nil {
		return true
	}
	return e.Watched.After(*f.Earliest)
}

// DiaryFilterLatest filters based on the latest date
func DiaryFilterLatest(e DiaryEntry, f DiaryFilterOpts) bool {
	if f.Latest == nil {
		return true
	}
	return e.Watched.Before(*f.Latest)
}

// DiaryFilterRewatch only show entries that are re-watches
func DiaryFilterRewatch(e DiaryEntry, f DiaryFilterOpts) bool {
	if f.Rewatch == nil {
		return true
	}
	return *f.Rewatch == e.Rewatch
}

// DiaryFilterMinRating filters based on minimum rating
func DiaryFilterMinRating(e DiaryEntry, f DiaryFilterOpts) bool {
	if f.MinRating == nil {
		return true
	}
	r := e.Rating
	fr := f.MinRating
	return *r >= *fr
}

// DiaryFilterMaxRating filters based on maximum rating
func DiaryFilterMaxRating(e DiaryEntry, f DiaryFilterOpts) bool {
	if f.MaxRating == nil {
		return true
	}
	r := e.Rating
	fr := f.MaxRating
	return *r <= *fr
}

// DiaryFilterDateSpecified only returns items that actually list the date they were watched
func DiaryFilterDateSpecified(e DiaryEntry, f DiaryFilterOpts) bool {
	if f.SpecifiedDate == nil {
		return true
	}
	return *f.SpecifiedDate == e.SpecifiedDate
}

// ApplyDiaryFilters applies all of the given filters to a given diary
func ApplyDiaryFilters(records DiaryEntries, opts DiaryFilterOpts, filters ...DiaryFilter) DiaryEntries {
	// Make sure there are actually filters to be applied.
	if len(filters) == 0 {
		return records
	}

	filteredRecords := make(DiaryEntries, 0, len(records))

	// Range over the records and apply all the filters to each record.
	// If the record passes all the filters, add it to the final slice.
	for _, r := range records {
		keep := true

		for _, f := range filters {
			if !f(*r, opts) {
				keep = false
				break
			}
		}

		if keep {
			filteredRecords = append(filteredRecords, r)
		}
	}

	return filteredRecords
}

// DiaryCobraOpts allows customization of the options passed in to Cobra Cmd
type DiaryCobraOpts struct {
	Prefix string
}

func prefixWithDiaryCobraOpts(opts DiaryCobraOpts) string {
	var prefix string
	if opts.Prefix != "" {
		prefix = opts.Prefix + "-"
	}
	return prefix
}

// BindDiaryFilterWithCobra inits all the pieces in a given Cmd to allow for diary filters
func BindDiaryFilterWithCobra(cmd *cobra.Command, opts DiaryCobraOpts) {
	prefix := prefixWithDiaryCobraOpts(opts)
	cmd.PersistentFlags().String(prefix+"earliest", "", "Earliest diary entries")
	cmd.PersistentFlags().String(prefix+"latest", "", "Latest diary entries")
	cmd.PersistentFlags().String(prefix+"year", "", "Only use entries from the given year")
	cmd.PersistentFlags().Int(prefix+"min-rating", 0, "Minimum rating for entries")
	cmd.PersistentFlags().Int(prefix+"max-rating", 10, "Maximum rating for entries")
	cmd.PersistentFlags().Bool(prefix+"rewatched", false, "Only return re-watched entries")
	cmd.PersistentFlags().Bool(prefix+"date-specified", false, "Only return entries with a date specified")
	cmd.MarkFlagsMutuallyExclusive(prefix+"year", prefix+"earliest")
	cmd.MarkFlagsMutuallyExclusive(prefix+"year", prefix+"latest")
}

// DiaryFilterWithCobra returns a diary filter from a Cobra Cmd
func DiaryFilterWithCobra(cmd *cobra.Command, dopts DiaryCobraOpts) (*DiaryFilterOpts, error) {
	prefix := prefixWithDiaryCobraOpts(dopts)
	opts := &DiaryFilterOpts{}

	var err error
	opts.Earliest, err = timeWithCobraString(cmd, prefix+"earliest")
	if err != nil {
		return nil, err
	}
	opts.Latest, err = timeWithCobraString(cmd, prefix+"latest")
	if err != nil {
		return nil, err
	}

	// Rating
	mir, err := cmd.Flags().GetInt(prefix + "min-rating")
	if err != nil {
		return nil, err
	}
	opts.MinRating = &mir

	mar, err := cmd.Flags().GetInt(prefix + "max-rating")
	if err != nil {
		return nil, err
	} else if mar > 0 {
		opts.MaxRating = &mar
	}

	yearS, err := cmd.PersistentFlags().GetString(prefix + "year")
	if err != nil {
		return nil, err
	} else if yearS != "" {
		year, err := strconv.Atoi(yearS)
		if err != nil {
			return nil, err
		}

		e := time.Date(year, time.Month(1), 1, 0, 0, 0, 0, time.UTC)
		l := time.Date(year+1, time.Month(1), 0, 0, 0, 0, 0, time.UTC)
		opts.Earliest = &e
		opts.Latest = &l
	}

	if cmd.PersistentFlags().Changed(prefix + "rewatched") {
		rewatched, err := cmd.Flags().GetBool(prefix + "rewatched")
		if err != nil {
			return nil, err
		}
		opts.Rewatch = &rewatched
	}

	if cmd.PersistentFlags().Changed(prefix + "date-specified") {
		dateSpecified, err := cmd.Flags().GetBool(prefix + "date-specified")
		if err != nil {
			return nil, err
		}
		opts.SpecifiedDate = &dateSpecified
	}
	return opts, nil
}

func timeWithCobraString(cmd *cobra.Command, s string) (*time.Time, error) {
	earliestS, err := cmd.Flags().GetString(s)
	if err != nil {
		return nil, err
	}
	if earliestS == "" {
		return nil, nil
	}
	formats := []string{
		"2006-01-02",
		"2006-01",
		"2006",
	}
	for _, format := range formats {
		t, err := time.Parse(format, earliestS)
		if err == nil {
			return &t, nil
		}
	}
	return nil, errors.New("could not parse in to a time")
}
