package letterboxd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestURLFilmographyBadProfession(t *testing.T) {
	_, err := sc.URL.Items(context.TODO(), "/televangelist/nicolas-cage")
	require.Error(t, err)
}

func TestURLFilmographyActor(t *testing.T) {
	items, err := sc.URL.Items(context.TODO(), "/actor/nicolas-cage")
	require.NoError(t, err)
	require.IsType(t, FilmSet{}, items)
	require.Greater(t, len(items.(FilmSet)), 0)
}

func TestURLWatchlist(t *testing.T) {
	items, err := sc.URL.Items(context.TODO(), "/singleguy/watchlist")
	require.NoError(t, err)
	require.IsType(t, FilmSet{}, items)
	require.Greater(t, len(items.(FilmSet)), 0)
}

func TestURLList(t *testing.T) {
	items, err := sc.URL.Items(context.TODO(), "/dave/list/official-top-250-narrative-feature-films")
	require.NoError(t, err)
	require.IsType(t, FilmSet{}, items)
	require.Greater(t, len(items.(FilmSet)), 0)
}

func TestURLFilms(t *testing.T) {
	items, err := sc.URL.Items(context.TODO(), "/singleguy/films")
	require.NoError(t, err)
	require.IsType(t, FilmSet{}, items)
	require.Greater(t, len(items.(FilmSet)), 0)
}

func TestNormalizeURLPath(t *testing.T) {
	tests := []struct {
		ourl         string
		expectedPath string
		wantErr      bool
		msg          string
	}{
		{ourl: "/film/everything-everywhere-all-at-once/", expectedPath: "/film/everything-everywhere-all-at-once", wantErr: false, msg: "no trailing slash"},
		{ourl: "/film/everything-everywhere-all-at-once", expectedPath: "/film/everything-everywhere-all-at-once", wantErr: false, msg: "trailing slash"},
		{ourl: "https://letterboxd.com/film/everything-everywhere-all-at-once/", expectedPath: "/film/everything-everywhere-all-at-once", wantErr: false, msg: "bare hostname"},
		{ourl: "https://www.letterboxd.com/film/everything-everywhere-all-at-once/", expectedPath: "/film/everything-everywhere-all-at-once", wantErr: false, msg: "www hostname"},
		{ourl: "https://www.google.com/film/everything-everywhere-all-at-once/", expectedPath: "", wantErr: true, msg: "invalid hostname"},
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
