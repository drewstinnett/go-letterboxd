package letterboxd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestURLFilmographyBadProfession(t *testing.T) {
	client := NewClient(nil)
	_, err := client.URL.Items(nil, "/televangelist/nicolas-cage")
	require.Error(t, err)
}

func TestURLFilmographyActor(t *testing.T) {
	items, err := sc.URL.Items(nil, "/actor/nicolas-cage")
	require.NoError(t, err)
	require.IsType(t, []*Film{}, items)
	require.Greater(t, len(items.([]*Film)), 0)
}

func TestNormalizeURLPath(t *testing.T) {
	tests := []struct {
		ourl         string
		expectedPath string
		wantErr      bool
		msg          string
	}{
		{"/film/everything-everywhere-all-at-once/", "/film/everything-everywhere-all-at-once", false, "no trailing slash"},
		{"/film/everything-everywhere-all-at-once", "/film/everything-everywhere-all-at-once", false, "trailing slash"},
		{"https://letterboxd.com/film/everything-everywhere-all-at-once/", "/film/everything-everywhere-all-at-once", false, "bare hostname"},
		{"https://www.letterboxd.com/film/everything-everywhere-all-at-once/", "/film/everything-everywhere-all-at-once", false, "www hostname"},
		{"https://www.google.com/film/everything-everywhere-all-at-once/", "", true, "invalid hostname"},
	}
	for _, tt := range tests {
		path, err := normalizeURLPath(tt.ourl)
		if tt.wantErr {
			require.Error(t, err, tt.msg)
		} else {
			require.NoError(t, err)
			require.Equal(t, tt.expectedPath, path, tt.msg)
		}
	}
}

/*
func TestURLWatchList(t *testing.T) {
	log.Warn("Starting watchlist")
	items, err := sc.URL.Items(context.TODO(), "/someguy/watchlist")
	log.Warn("GOT watchlist")
	require.NoError(t, err)
	require.IsType(t, []*Film{}, items)
	require.Greater(t, len(items.([]*Film)), 0)
}
*/
