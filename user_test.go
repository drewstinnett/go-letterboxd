package letterboxd

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractUserFilms(t *testing.T) {
	f, err := os.Open("testdata/user/films.html")
	require.NoError(t, err)
	defer f.Close()

	items, _, err := ExtractUserFilms(f)
	films := items.([]*Film)
	require.NoError(t, err)
	require.Greater(t, len(films), 70)
	require.Equal(t, "Cypress Hill: Insane in the Brain", films[0].Title)
}

func TestExtractUserFilmsSinglePage(t *testing.T) {
	f, err := os.Open("testdata/user/watched-films-single.html")
	require.NoError(t, err)
	defer f.Close()

	items, _, err := ExtractUserFilms(f)
	require.NoError(t, err)
	films := items.([]*Film)
	require.Equal(t, len(films), 34)
	require.Equal(t, "Irresistible", films[0].Title)
}

func TestExtractUser(t *testing.T) {
	f, err := os.Open("testdata/user/user.html")
	require.NoError(t, err)
	defer f.Close()
	user, _, err := ExtractUser(f)
	require.NoError(t, err)
	require.IsType(t, &User{}, user)
	u := user.(*User)
	require.Equal(t, "dankmccoy", u.Username)
	require.Equal(t, "Former writer for The Daily Show with Jon Stewart (also Trevor Noah). Podcaster -- The Flop House. I watch a lot of trash, but I also care about good stuff, I swear.", u.Bio)
}

func TestUserProfile(t *testing.T) {
	item, _, err := sc.User.Profile(context.TODO(), "someguy")
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

		item, _, err := sc.User.Profile(context.TODO(), tt.user)
		if tt.expect {
			require.NoError(t, err)
			require.IsType(t, &User{}, item)
		} else {
			require.Error(t, err)
		}
	}
}

func TestStreamWatchedWithChan(t *testing.T) {
	watchedC := make(chan *Film)
	done := make(chan error)
	go sc.User.StreamWatched(context.TODO(), "someguy", watchedC, done)
	watched, err := SlurpFilms(watchedC, done)
	require.NoError(t, err)
	require.NotEmpty(t, watched)
	require.Equal(t, 321, len(watched))
}

func TestStreamListWithChan(t *testing.T) {
	watchedC := make(chan *Film)
	var watched []*Film
	done := make(chan error)
	go sc.User.StreamList(context.TODO(), "dave", "official-top-250-narrative-feature-films", watchedC, done)
	watched, err := SlurpFilms(watchedC, done)
	require.NoError(t, err)

	require.NotEmpty(t, watched)
	require.Equal(t, 250, len(watched))
}

func TestExtractUserDiary(t *testing.T) {
	data, err := os.ReadFile("testdata/user/diary-paginated/1.html")
	require.NoError(t, err)

	itemsI, _, err := sc.User.ExtractDiaryEntries(bytes.NewReader(data))
	items := itemsI.([]*DiaryEntry)
	require.NoError(t, err)
	require.Equal(t, len(items), 50)
	require.Equal(t, 7, *items[0].Rating)
	require.Equal(t, "cure", *items[0].Slug)

	require.NotNil(t, items[0].Film)
	require.Equal(t, "Sweet Sweetback's Baadasssss Song", items[0].Film.Title)
}

func TestStreamDiaryWithChan(t *testing.T) {
	diaryC := make(chan *DiaryEntry)
	doneC := make(chan error)
	go sc.User.StreamDiary(context.TODO(), "someguy", diaryC, doneC)
	items, err := SlurpDiary(diaryC, doneC)
	require.NoError(t, err)
	require.NotEmpty(t, items)
	require.Equal(t, 175, len(items))
}

func TestGetDiary(t *testing.T) {
	items, err := sc.User.GetDiary(context.Background(), "someguy")
	require.NoError(t, err)
	require.Equal(t, 175, len(items))
}
