package letterboxd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redismock/v8"
)

var (
	srv     *httptest.Server
	sc      *Client
	scc     *Client
	sccMock redismock.ClientMock
)

// FileToResponseWriter is a helper utility to load a page right in to the http response
func FileToResponseWriter(f string, w http.ResponseWriter) {
	rp, err := os.ReadFile(f)
	panicIfErr(err)
	_, err = io.Copy(w, bytes.NewReader(rp))
	panicIfErr(err)
}

func setup() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.With().Caller().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/dave/list/official-top-250-narrative-feature-films/page/"):
			pageNo := strings.Split(r.URL.Path, "/")[5]
			FileToResponseWriter(fmt.Sprintf("testdata/list/lists-page-%v.html", pageNo), w)
		case strings.HasPrefix(r.URL.Path, "/films/ajax/popular/size/"):
			FileToResponseWriter("testdata/films/popular.html", w)
		case strings.HasPrefix(r.URL.Path, "/singleguy/films"):
			FileToResponseWriter("testdata/user/films-single.html", w)
		case strings.HasPrefix(r.URL.Path, "/film/"):
			FileToResponseWriter("testdata/film/sweetback.html", w)
		case strings.Contains(r.URL.Path, "/actor/nicolas-cage"):
			FileToResponseWriter("testdata/filmography/actor/nicolas-cage.html", w)
		case strings.Contains(r.URL.Path, "singleguy/watchlist"):
			FileToResponseWriter("testdata/user/watchlist-single.html", w)
		case strings.Contains(r.URL.Path, "/someguy/films/page/"):
			pageNo := strings.Split(r.URL.Path, "/")[4]
			FileToResponseWriter(fmt.Sprintf("testdata/user/watched-paginated/%v.html", pageNo), w)
		case strings.Contains(r.URL.Path, "/someguy/following/page/"):
			pageNo := strings.Split(r.URL.Path, "/")[4]
			FileToResponseWriter(fmt.Sprintf("testdata/user/following/%v.html", pageNo), w)
		case strings.Contains(r.URL.Path, "/someguy/followers/page/"):
			pageNo := strings.Split(r.URL.Path, "/")[4]
			FileToResponseWriter(fmt.Sprintf("testdata/user/followers/%v.html", pageNo), w)
		case strings.Contains(r.URL.Path, "/someguy/films/diary/"):
			pageNo := strings.Split(r.URL.Path, "/")[5]
			FileToResponseWriter(fmt.Sprintf("testdata/user/diary-paginated/%v.html", pageNo), w)
		case strings.Contains(r.URL.Path, "someguy/watchlist/page/"):
			FileToResponseWriter("testdata/user/watchlist.html", w)
			return
		case r.URL.Path == "/someguy":
			FileToResponseWriter("testdata/user/user.html", w)
		default:
			log.Warn().
				Str("url", r.URL.String()).
				Msg("unexpected request")
			w.WriteHeader(http.StatusNotFound)
		}
		defer r.Body.Close()
	}))

	// Non-Caching Client
	sc = New(
		WithNoCache(),
		WithBaseURL(srv.URL),
	)

	// Caching Client
	var db *redis.Client
	db, sccMock = redismock.NewClientMock()
	scc = New(
		WithCache(
			cache.New(&cache.Options{
				Redis: db,
			}),
		),
		WithBaseURL(srv.URL),
	)
}

func shutdown() {
	srv.Close()
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	shutdown()
	os.Exit(code)
}

func TestMustNewRequest(t *testing.T) {
	require.NotNil(t, mustNewRequest("GET", "https://www.example.com", nil))
	require.NotNil(t, mustNewGetRequest("https://www.example.com"))
}

func TestNew(t *testing.T) {
	c := New()
	require.NotNil(t, c)
}
