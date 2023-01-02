package letterboxd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeStartStop(t *testing.T) {
	tests := []struct {
		firstPage, lastPage                 int
		expectedFirstPage, expectedLastPage int
		expectErr                           bool
		msg                                 string
	}{
		{0, 0, 1, 1, false, "first page and last page are 0"},
		{0, 1, 1, 1, false, "first page is 0"},
		{1, 0, 1, 1, false, "last page is 0"},
		{1, 1, 1, 1, false, "first and last page are 1"},
		{1, 2, 1, 2, false, "first and last page are 1 and 2"},
		{2, 2, 2, 2, false, "first and last page are 2 and 2"},
		{2, 1, 0, 0, true, "first page is greater than last page"},
		{1, -1, 1, -1, false, "first page is 1 and last page is -1"},
	}
	for _, tt := range tests {
		firstPage, lastPage, err := normalizeStartStop(tt.firstPage, tt.lastPage)
		if tt.expectErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err, tt.msg)
			require.Equal(t, tt.expectedFirstPage, firstPage, tt.msg)
			require.Equal(t, tt.expectedLastPage, lastPage, tt.msg)
		}
	}
}

func TestNormalizeSlug(t *testing.T) {
	tests := []struct {
		slug         string
		expectedSlug string
	}{
		{"/film/everything-everywhere-all-at-once", "everything-everywhere-all-at-once"},
		{"/film/everything-everywhere-all-at-once/", "everything-everywhere-all-at-once"},
	}
	for _, tt := range tests {
		slug := normalizeSlug(tt.slug)
		require.Equal(t, tt.expectedSlug, slug)
	}
}

func TestParseListArgs(t *testing.T) {
	tests := []struct {
		args    []string
		want    []*ListID
		wantErr bool
	}{
		{
			[]string{"foo/bar"},
			[]*ListID{
				{
					User: "foo",
					Slug: "bar",
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		got, err := ParseListArgs(tt.args)
		if tt.wantErr {
			require.Error(t, err)
		} else {
			require.Equal(t, tt.want, got)
		}
	}
}

func TestPopulateRemainingPages(t *testing.T) {
	tests := map[string]struct {
		count   int
		total   int
		shuffle bool
		want    []int
	}{
		"plain": {
			count: 5, total: 7, shuffle: false, want: []int{2, 3, 4, 5, 6},
		},
	}

	for k, tt := range tests {
		got := populateRemainingPages(tt.count, tt.total, tt.shuffle)
		require.Equal(t, tt.want, got, k)
	}
}

func TestPopulateRemainingPagesShuffle(t *testing.T) {
	got := populateRemainingPages(5, 7, true)
	require.NotNil(t, got)
	require.Equal(t, 6, len(got))
}

func TestMin(t *testing.T) {
	got := min(2, 1, 3)
	require.Equal(t, 1, got)

	require.Panics(t, func() { min() })
}

func TestMax(t *testing.T) {
	got := max(2, 3, 1)
	require.Equal(t, 3, got)

	require.Panics(t, func() { max() })
}

func TestStringOr(t *testing.T) {
	require.Equal(t, "given", stringOr("given", "default"))
	require.Equal(t, "default", stringOr("", "default"))
}
