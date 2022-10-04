package letterboxd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractUserFilms(t *testing.T) {
	f, err := os.Open("testdata/user/films.html")
	defer f.Close()
	require.NoError(t, err)

	items, _, err := ExtractUserFilms(f)
	films := items.([]*Film)
	require.NoError(t, err)
	require.Greater(t, len(films), 70)
	require.Equal(t, "Cypress Hill: Insane in the Brain", films[0].Title)
}

func TestExtractUserFilmsSinglePage(t *testing.T) {
	f, err := os.Open("testdata/user/watched-films-single.html")
	defer f.Close()
	require.NoError(t, err)

	items, _, err := ExtractUserFilms(f)
	require.NoError(t, err)
	films := items.([]*Film)
	require.Equal(t, len(films), 34)
	require.Equal(t, "Irresistible", films[0].Title)
}

func TestExtractUser(t *testing.T) {
	f, err := os.Open("testdata/user/user.html")
	defer f.Close()
	require.NoError(t, err)
	user, _, err := ExtractUser(f)
	require.NoError(t, err)
	require.IsType(t, &User{}, user)
	u := user.(*User)
	require.Equal(t, "dankmccoy", u.Username)
	require.Equal(t, "Former writer for The Daily Show with Jon Stewart (also Trevor Noah). Podcaster -- The Flop House. I watch a lot of trash, but I also care about good stuff, I swear.", u.Bio)
}

func TestUserProfile(t *testing.T) {
	item, _, err := sc.User.Profile(nil, "someguy")
	require.NoError(t, err)
	require.IsType(t, &User{}, item)
	require.Equal(t, 1398, item.WatchedFilmCount)
}

func TestUserProfileExists(t *testing.T) {
	tests := []struct {
		user   string
		expect bool
	}{
		{user: "someguy", expect: true},
		{user: "neverexist", expect: false},
	}
	for _, tt := range tests {

		item, _, err := sc.User.Profile(nil, tt.user)
		if tt.expect {
			require.NoError(t, err)
			require.IsType(t, &User{}, item)
		} else {
			require.Error(t, err)
		}
	}
}

/*
func TestListWatched(t *testing.T) {
	watched, _, err := sc.User.Watched(nil, "someguy")
	require.NoError(t, err)
	require.NotNil(t, watched)

	require.Equal(t, 321, len(watched))
}
*/

func TestStreamWatchedWithChan(t *testing.T) {
	watchedC := make(chan *Film, 0)
	done := make(chan error)
	go sc.User.StreamWatched(nil, "someguy", watchedC, done)
	watched, err := SlurpFilms(watchedC, done)
	require.NoError(t, err)
	require.NotEmpty(t, watched)
	require.Equal(t, 321, len(watched))
}

func TestStreamListWithChan(t *testing.T) {
	watchedC := make(chan *Film, 0)
	var watched []*Film
	done := make(chan error)
	go sc.User.StreamList(nil, "dave", "official-top-250-narrative-feature-films", watchedC, done)
	watched, err := SlurpFilms(watchedC, done)
	require.NoError(t, err)

	require.NotEmpty(t, watched)
	require.Equal(t, 250, len(watched))
}
