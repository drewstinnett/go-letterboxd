package letterboxd

import (
	"errors"
	"time"

	"github.com/spf13/cobra"
)

type DiaryEntry struct {
	Watched       *time.Time
	Rating        *int
	Rewatch       bool
	SpecifiedDate bool
	Film          *Film
	Slug          *string
}

type DiaryEntries []DiaryEntry

type DiaryFilterOpts struct {
	Earliest      *time.Time
	Latest        *time.Time
	Rewatch       *bool
	SpecifiedDate *bool
}

type (
	DiaryFilter     func(DiaryEntry, DiaryFilterOpts) bool
	DiaryFilterBulk func(DiaryEntries, DiaryFilterOpts) DiaryEntries
)

func DiaryFilterEarliest(e DiaryEntry, f DiaryFilterOpts) bool {
	if f.Earliest == nil {
		return true
	}
	return e.Watched.After(*f.Earliest)
}

func DiaryFilterLatest(e DiaryEntry, f DiaryFilterOpts) bool {
	if f.Latest == nil {
		return true
	}
	return e.Watched.Before(*f.Latest)
}

func DiaryFilterRewatch(e DiaryEntry, f DiaryFilterOpts) bool {
	if f.Rewatch == nil {
		return true
	}
	return *f.Rewatch == e.Rewatch
}

func DiaryFilterDateSpecified(e DiaryEntry, f DiaryFilterOpts) bool {
	if f.SpecifiedDate == nil {
		return true
	}
	return *f.SpecifiedDate == e.SpecifiedDate
}

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
			if !f(r, opts) {
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

func BindDiaryFilterWithCobra(cmd *cobra.Command) {
	cmd.PersistentFlags().String("diary-earliest", "", "Earliest diary entries")
	cmd.PersistentFlags().String("diary-latest", "", "Latest diary entries")
	cmd.PersistentFlags().Bool("diary-rewatched", false, "Only return re-watched entries")
	cmd.PersistentFlags().Bool("diary-date-specified", false, "Only return entries with a date specified")
}

func DiaryFilterWithCobra(cmd *cobra.Command) (*DiaryFilterOpts, error) {
	var err error
	opts := &DiaryFilterOpts{}

	opts.Earliest, err = timeWithCobraString(cmd, "diary-earliest")
	if err != nil {
		return nil, err
	}
	opts.Latest, err = timeWithCobraString(cmd, "diary-latest")
	if err != nil {
		return nil, err
	}
	rewatched, err := cmd.Flags().GetBool("diary-rewatched")
	if err != nil {
		return nil, err
	}
	opts.Rewatch = &rewatched
	dateSpecified, err := cmd.Flags().GetBool("diary-date-specified")
	if err != nil {
		return nil, err
	}
	opts.SpecifiedDate = &dateSpecified
	return opts, nil
}

func timeWithCobraString(cmd *cobra.Command, s string) (*time.Time, error) {
	earliestS, err := cmd.Flags().GetString(s)
	if err != nil {
		return nil, err
	}
	formats := []string{
		"2006-01-02",
		"2006-01",
		"2006",
	}
	for _, format := range formats {
		t, err := time.Parse(format, earliestS)
		if err != nil {
			return &t, nil
		}
	}
	return nil, errors.New("Could not parse in to a time")
}
