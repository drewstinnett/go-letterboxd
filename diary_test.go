package letterboxd

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestExtractUserDiary(t *testing.T) {
	data, err := os.ReadFile("testdata/user/diary-paginated/1.html")
	require.NoError(t, err)

	itemsI, _, err := sc.User.ExtractDiaryEntries(bytes.NewReader(data))
	items := itemsI.([]*DiaryEntry)
	require.NoError(t, err)
	require.Equal(t, len(items), 50)
	require.Equal(t, 7, *items[0].Rating)
	require.Equal(t, "cure", *items[0].Slug)
	require.Equal(t, true, items[0].SpecifiedDate)
	require.Equal(t, true, items[0].Rewatch)

	require.NotNil(t, items[0].Film)
	require.Equal(t, "Sweet Sweetback's Baadasssss Song", items[0].Film.Title)
}

func TestFilterEarliest(t *testing.T) {
	require.Equal(t, true, DiaryFilterEarliest(DiaryEntry{}, DiaryFilterOpts{}))

	et, _ := time.Parse("2006-01-02", "2020-01-29")
	e := DiaryEntry{
		Watched: &et,
	}
	ft, _ := time.Parse("2006-01-02", "2021-01-29")
	f := DiaryFilterOpts{
		Earliest: &ft,
	}
	require.Equal(t, false, DiaryFilterEarliest(e, f))
}

func TestFilterLatest(t *testing.T) {
	require.Equal(t, true, DiaryFilterLatest(DiaryEntry{}, DiaryFilterOpts{}))

	et, _ := time.Parse("2006-01-02", "2020-01-29")
	e := DiaryEntry{
		Watched: &et,
	}
	ft, _ := time.Parse("2006-01-02", "2019-01-29")
	f := DiaryFilterOpts{
		Latest: &ft,
	}
	require.Equal(t, false, DiaryFilterLatest(e, f))
}

func TestFilterRewatch(t *testing.T) {
	truthy := true
	require.Equal(t, true, DiaryFilterRewatch(DiaryEntry{}, DiaryFilterOpts{}))
	require.Equal(t, true, DiaryFilterRewatch(
		DiaryEntry{
			Rewatch: true,
		},
		DiaryFilterOpts{
			Rewatch: &truthy,
		},
	))
}

func TestFilterSpecifiedDate(t *testing.T) {
	truthy := true
	require.Equal(t, true, DiaryFilterDateSpecified(DiaryEntry{}, DiaryFilterOpts{}))
	require.Equal(t, true, DiaryFilterDateSpecified(
		DiaryEntry{
			SpecifiedDate: true,
		},
		DiaryFilterOpts{
			SpecifiedDate: &truthy,
		},
	))
}

func TestDiaryFilterMinRating(t *testing.T) {
	r := 7
	fr := 5
	require.Equal(t, true, DiaryFilterMinRating(DiaryEntry{}, DiaryFilterOpts{}))
	require.Equal(t, true, DiaryFilterMinRating(
		DiaryEntry{
			Rating: &r,
		},
		DiaryFilterOpts{
			MinRating: &fr,
		},
	))
}

func TestDiaryFilterMaxRating(t *testing.T) {
	r := 5
	fr := 7
	require.Equal(t, true, DiaryFilterMaxRating(DiaryEntry{}, DiaryFilterOpts{}))
	require.Equal(t, true, DiaryFilterMaxRating(
		DiaryEntry{
			Rating: &r,
		},
		DiaryFilterOpts{
			MaxRating: &fr,
		},
	))
}

func TestApplyDiaryFilters(t *testing.T) {
	t1, _ := time.Parse("2006-01-02", "2019-01-29")
	t2, _ := time.Parse("2006-01-02", "2021-01-29")
	ft, _ := time.Parse("2006-01-02", "2020-01-29")
	got := ApplyDiaryFilters(
		DiaryEntries{
			{Watched: &t1},
			{Watched: &t2},
		},
		DiaryFilterOpts{
			Earliest: &ft,
		},
		DiaryFilterEarliest)
	require.Equal(t, 1, len(got))
}

func TestDiaryFilterWithCobra(t *testing.T) {
	cmd := &cobra.Command{}
	BindDiaryFilterWithCobra(cmd, DiaryCobraOpts{})
	f, err := DiaryFilterWithCobra(cmd, DiaryCobraOpts{})
	require.NoError(t, err)
	require.NotNil(t, f)
}

func TestDiaryFilterWithCobraWithPrefix(t *testing.T) {
	cmd := &cobra.Command{}
	opts := DiaryCobraOpts{
		Prefix: "foo",
	}
	BindDiaryFilterWithCobra(cmd, opts)
	f, err := DiaryFilterWithCobra(cmd, opts)
	require.NoError(t, err)
	require.NotNil(t, f)
}

func TestPrefixWithCobraOpts(t *testing.T) {
	tests := map[string]struct {
		opts DiaryCobraOpts
		want string
	}{
		"empty": {
			opts: DiaryCobraOpts{},
			want: "",
		},
		"set-value": {
			opts: DiaryCobraOpts{
				Prefix: "hello",
			},
			want: "hello-",
		},
	}
	for desc, tt := range tests {
		require.Equal(t, tt.want, prefixWithDiaryCobraOpts(tt.opts), desc)
	}
}
