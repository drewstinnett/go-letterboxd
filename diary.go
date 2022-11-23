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
