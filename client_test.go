package letterboxd

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog/log"
)

var (
	srv *httptest.Server
	sc  *Client
)

func setup() {
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/dave/list/official-top-250-narrative-feature-films/page/") {
			pageNo := strings.Split(r.URL.Path, "/")[5]
			rp, err := os.Open(fmt.Sprintf("testdata/list/lists-page-%v.html", pageNo))
			if err != nil {
				panic(err)
			}
			defer rp.Close()
			_, err = io.Copy(w, rp)
			if err != nil {
				panic(err)
			}
			return
		} else if strings.HasPrefix(r.URL.Path, "/film/") {
			sweetbackF, err := os.Open("testdata/film/sweetback.html")
			if err != nil {
				panic(err)
			}
			defer sweetbackF.Close()

			_, err = io.Copy(w, sweetbackF)
			if err != nil {
				panic(err)
			}
			return
		} else if strings.Contains(r.URL.Path, "/actor/nicolas-cage") {
			rp, err := os.Open("testdata/filmography/actor/nicolas-cage.html")
			if err != nil {
				panic(err)
			}
			defer rp.Close()

			_, err = io.Copy(w, rp)
			if err != nil {
				panic(err)
			}
			return
		} else if strings.Contains(r.URL.Path, "/someguy/films/page/") {
			pageNo := strings.Split(r.URL.Path, "/")[4]
			rp, err := os.Open(fmt.Sprintf("testdata/user/watched-paginated/%v.html", pageNo))
			if err != nil {
				panic(err)
			}
			defer rp.Close()
			_, err = io.Copy(w, rp)
			if err != nil {
				panic(err)
			}
			return
		} else if strings.Contains(r.URL.Path, "someguy/watchlist/page/") {
			rp, err := os.Open(fmt.Sprintf("testdata/user/watchlist.html"))
			if err != nil {
				panic(err)
			}
			defer rp.Close()
			_, err = io.Copy(w, rp)
			if err != nil {
				panic(err)
			}
			return
		} else if r.URL.Path == "/someguy" {
			f, err := os.Open("testdata/user/user.html")
			if err != nil {
				panic(err)
			}
			defer f.Close()
			io.Copy(w, f)
		} else {
			log.Warn().
				Str("url", r.URL.String()).
				Msg("unexpected request")
			w.WriteHeader(http.StatusNotFound)
		}
		defer r.Body.Close()
	}))
	sc = NewClient(nil)
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
