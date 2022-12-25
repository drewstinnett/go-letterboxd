package letterboxd

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExtractIDFromURL(t *testing.T) {
	tests := []struct {
		url string
		id  string
	}{
		{url: "http://www.imdb.com/title/tt0067810/maindetails", id: "tt0067810"},
		{url: "https://www.themoviedb.org/movie/5822/", id: "5822"},
		{url: "https://www.google.com", id: ""},
	}
	for _, tt := range tests {
		id := extractIDFromURL(tt.url)
		require.Equal(t, tt.id, id)
	}
}

func TestExtractFilmFromFilmPage(t *testing.T) {
	f, err := os.Open("testdata/film/sweetback.html")
	require.NoError(t, err)
	defer f.Close()
	i, pagination, err := extractFilmFromFilmPage(f)
	film := i.(*Film)
	require.NoError(t, err)
	require.Nil(t, pagination)
	require.NotNil(t, film)
	require.NotNil(t, film.ExternalIDs)
	require.Equal(t, "tt0067810", film.ExternalIDs.IMDB)
	require.Equal(t, "5822", film.ExternalIDs.TMDB)
	require.Equal(t, "Sweet Sweetback's Baadasssss Song", film.Title)
	require.Equal(t, "sweet-sweetbacks-baadasssss-song", film.Slug)
	require.Equal(t, "/film/sweet-sweetbacks-baadasssss-song/", film.Target)
	require.Equal(t, "48640", film.ID)
}

func TestEnhanceFilmList(t *testing.T) {
	// Make sure we don't get the external ids on a normal call
	// require.Nil(t, films[0].ExternalIDs)
	films := FilmSet{
		{
			Slug: "sweet-sweetbacks-baadasssss-song",
		},
	}

	// Make sure we DO get them after enhancing
	err := sc.Film.EnhanceFilmList(context.TODO(), &films)
	require.NoError(t, err)
	require.NotNil(t, films[0].ExternalIDs)
}

func TestFilmography(t *testing.T) {
	profession := "actor"
	person := "nicolas-cage"
	films, err := sc.Film.Filmography(context.TODO(), &FilmographyOpt{
		Person:     person,
		Profession: profession,
	})
	require.NoError(t, err)
	require.NotNil(t, films)
	require.Equal(t, 116, len(films))
	require.Equal(t, "Spider-Man: Into the Spider-Verse", films[0].Title)
}

func TestValidateFilmography(t *testing.T) {
	tests := []struct {
		opt     FilmographyOpt
		wantErr bool
	}{
		{FilmographyOpt{
			Profession: "actor",
		}, true},
		{FilmographyOpt{
			Person: "John Doe",
		}, true},
		{FilmographyOpt{
			Person:     "John Doe",
			Profession: "wait-staff",
		}, true},
		{FilmographyOpt{
			Person:     "nicolas-cage",
			Profession: "actor",
		}, false},
	}
	for _, tt := range tests {
		got := tt.opt.Validate()
		if tt.wantErr {
			require.Error(t, got)
		} else {
			require.NoError(t, got)
		}
	}
}

func TestStreamBatchWithChan(t *testing.T) {
	watchedC := make(chan *Film)
	errorC := make(chan error)
	go sc.Film.StreamBatch(context.TODO(), &FilmBatchOpts{
		Watched: []string{"someguy"},
		List: []*ListID{
			{User: "dave", Slug: "official-top-250-narrative-feature-films"},
		},
		WatchList: []string{"someguy"},
	}, watchedC, errorC)
	watched, err := SlurpFilms(watchedC, errorC)
	require.NoError(t, err)

	require.NotEmpty(t, watched)
	require.Equal(t, 655, len(watched))
}

func TestFilmGet(t *testing.T) {
	film, err := sc.Film.Get(context.TODO(), "sweet-sweetbacks-baadasssss-song")
	require.NoError(t, err)
	require.NotNil(t, film)
	require.Equal(t, "48640", film.ID)
	require.Equal(t, "Sweet Sweetback's Baadasssss Song", film.Title)
	require.Equal(t, 1971, film.Year)
	require.Equal(t, "tt0067810", film.ExternalIDs.IMDB)
	require.Equal(t, "5822", film.ExternalIDs.TMDB)
}

func TestExtractYearFromTitle(t *testing.T) {
	tests := []struct {
		title   string
		year    int
		wantErr bool
	}{
		{"Sweet Sweetback&#039;s Baadasssss Song (1971)", 1971, false},
		{"Sweet Sweetback&#039;s Baadasssss Song", 0, true},
		{"12345", 0, true},
		{"Sweet Sweetback&#039;s Baadasssss Song (abcd)", 0, true},
	}
	for _, tt := range tests {
		year, err := extractYearFromTitle(tt.title)
		if tt.wantErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tt.year, year)
		}
	}
}

func TestEnhanceFilm(t *testing.T) {
	ogFilm := &Film{
		Slug: "sweet-sweetbacks-baadasssss-song",
	}
	err := sc.Film.EnhanceFilm(context.TODO(), ogFilm)
	require.NoError(t, err)
	require.NotNil(t, ogFilm)
	require.Equal(t, 1971, ogFilm.Year)
	require.Equal(t, "Sweet Sweetback's Baadasssss Song", ogFilm.Title)
	require.Equal(t, "tt0067810", ogFilm.ExternalIDs.IMDB)
	require.Equal(t, "5822", ogFilm.ExternalIDs.TMDB)
	require.Equal(t, "48640", ogFilm.ID)
}

func TestFilmsList(t *testing.T) {
	got, err := sc.Film.List(context.Background(), &FilmListOpts{
		SortBy: "popular",
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotEmpty(t, got)
	require.Equal(t, 72, len(got))
}

func TestSendRequestCached(t *testing.T) {
	// First fetch should not be from the cache
	sccMock.ClearExpect()

	req := mustNewGetRequest("https://www.letterboxd.com/film/sweet-sweetbacks-baadasssss-song")

	key := "/letterboxd/fullpage/film/sweet-sweetbacks-baadasssss-song"
	sccMock.ExpectGet(key).RedisNil()
	sccMock.Regexp().ExpectSet(key, `.*`, time.Hour*24).SetVal("OK")
	_, resp, err := scc.sendRequest(req, extractFilmFromFilmPage)
	require.NoError(t, err)
	require.Equal(t, false, resp.FromCache)

	// Next one SHOULD be from the cache
	// sccMock.ExpectGet(key).SetVal("ok")
	// sccMock.ExpectGet(key).RedisNil()
	_, _, err = scc.sendRequest(req, extractFilmFromFilmPage)
	require.NoError(t, err)
	// require.Equal(t, true, resp.FromCache)
	require.NoError(t, sccMock.ExpectationsWereMet())
}
