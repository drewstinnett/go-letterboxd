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
)

var (
	srv *httptest.Server
	sc  *Client
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
		if strings.Contains(r.URL.Path, "/dave/list/official-top-250-narrative-feature-films/page/") {
			pageNo := strings.Split(r.URL.Path, "/")[5]
			FileToResponseWriter(fmt.Sprintf("testdata/list/lists-page-%v.html", pageNo), w)
			return
		} else if strings.HasPrefix(r.URL.Path, "/films/ajax/popular/size/") {
			FileToResponseWriter("testdata/films/popular.html", w)
			return
		} else if strings.HasPrefix(r.URL.Path, "/film/") {
			FileToResponseWriter("testdata/film/sweetback.html", w)
			return
		} else if strings.Contains(r.URL.Path, "/actor/nicolas-cage") {
			FileToResponseWriter("testdata/filmography/actor/nicolas-cage.html", w)
			return
		} else if strings.Contains(r.URL.Path, "/someguy/films/page/") {
			pageNo := strings.Split(r.URL.Path, "/")[4]
			FileToResponseWriter(fmt.Sprintf("testdata/user/watched-paginated/%v.html", pageNo), w)
			return
		} else if strings.Contains(r.URL.Path, "/someguy/films/diary/") {
			pageNo := strings.Split(r.URL.Path, "/")[5]
			FileToResponseWriter(fmt.Sprintf("testdata/user/diary-paginated/%v.html", pageNo), w)
			return
		} else if strings.Contains(r.URL.Path, "someguy/watchlist/page/") {
			FileToResponseWriter("testdata/user/watchlist.html", w)
			return
		} else if r.URL.Path == "/someguy" {
			FileToResponseWriter("testdata/user/user.html", w)
			return
		} else {
			log.Warn().
				Str("url", r.URL.String()).
				Msg("unexpected request")
			w.WriteHeader(http.StatusNotFound)
		}
		defer r.Body.Close()
	}))
	sc = NewClient(&ClientConfig{
		DisableCache: true,
	})
	sc.BaseURL = srv.URL
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

func TestMustNewRequst(t *testing.T) {
	got := MustNewRequest("GET", "https://www.example.com", nil)
	require.NotNil(t, got)
}
