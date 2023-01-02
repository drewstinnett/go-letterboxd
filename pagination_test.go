package letterboxd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
)

func TestExtractPagination(t *testing.T) {
	var pagination *Pagination
	f, err := os.Open("testdata/user/films.html")
	require.NoError(t, err)
	defer f.Close()
	doc, err := goquery.NewDocumentFromReader(f)
	require.NoError(t, err)

	pagination, err = ExtractPaginationWithDoc(doc)
	require.NoError(t, err)
	require.Equal(t, 1, pagination.CurrentPage)
	require.Equal(t, 59, pagination.TotalPages)
}

func TestExtractPaginationNoPagination(t *testing.T) {
	p, err := ExtractPagination(strings.NewReader("Just some text"))
	require.Nil(t, p)
	require.Error(t, err)
	require.EqualError(t, err, "could not extract pagination, no current page")
}

func TestExtractPaginationBytes(t *testing.T) {
	tests := []struct {
		content            []byte
		expectedPagination *Pagination
		expectedError      error
	}{
		{
			content: []byte(`<div class="paginate-pages">
			    <ul>
				<li class="paginate-page paginate-current">
				    <span>1</span></li>
				<li class="paginate-page">
				    <a href="/mondodrew/films/page/2/">2</a>
				</li>
				<li class="paginate-page">
				    <a href="/mondodrew/films/page/3/">3</a>
				</li>
				<li class="paginate-page unseen-pages">&hellip;</li>
				<li class="paginate-page">
				    <a href="/mondodrew/films/page/59/">59</a>
				</li>
			    </ul>
			</div>
		    </div>`),
			expectedPagination: &Pagination{
				CurrentPage: 1,
				NextPage:    2,
				TotalPages:  59,
			},
			expectedError: nil,
		},
		{
			content: []byte(`
<div class="pagination">
  <div class="paginate-nextprev">
    <a class="previous" href="/mondodrew/films/page/58/">Newer</a>
  </div>
  <div class="paginate- nextprev paginate-disabled">
    <span class="next">Older</span>
  </div>
  <div class="paginate-pages">
    <ul>
      <li class="paginate-page"><a href="/mondodrew/films/">1</a></li>
      <li class="pa ginate-page unseen-pages">&hellip;</li>
      <li class="paginate-page"><a href="/mondodrew/films/page/57/">57</a></li>
      <li class="paginate-page"><a href="/mondodrew/films/page/58/">58 </a></li>
      <li class="paginate-page paginate-current"><span>59</span></li>
    </ul>
  </div>
  </div>`),
			expectedPagination: &Pagination{
				CurrentPage: 59,
				NextPage:    0,
				TotalPages:  59,
				IsLast:      true,
			},
			expectedError: nil,
		},
	}
	for _, tt := range tests {
		pagination, err := ExtractPagination(bytes.NewReader(tt.content))
		require.Equal(t, tt.expectedError, err)
		require.Equal(t, tt.expectedPagination, pagination)
	}
}

func TestExtractPaginationOnFilmsPage(t *testing.T) {
	var pagination *Pagination
	f, err := os.Open("testdata/films/popular.html")
	require.NoError(t, err)
	defer f.Close()
	doc, err := goquery.NewDocumentFromReader(f)
	require.NoError(t, err)

	pagination, err = ExtractPaginationWithDoc(doc)
	require.NoError(t, err)
	require.Equal(t, 1, pagination.CurrentPage)
	require.Equal(t, 10412, pagination.TotalPages)
}

func TestExtractHasNext(t *testing.T) {
	b, err := os.Open("testdata/user/following/1.html")
	require.NoError(t, err)
	defer b.Close()
	got := hasNext(b)
	require.True(t, got)
}

func TestExtractNotHasNext(t *testing.T) {
	b, err := os.Open("testdata/user/following/2.html")
	require.NoError(t, err)
	defer b.Close()
	got := hasNext(b)
	require.False(t, got)
}

func TestExtractHasNextBytes(t *testing.T) {
	b, err := os.ReadFile("testdata/user/following/1.html")
	require.NoError(t, err)
	got := hasNext(bytes.NewReader(b))
	require.True(t, got)
}

func TestExtractNotHasNextBytes(t *testing.T) {
	b, err := os.ReadFile("testdata/user/following/2.html")
	require.NoError(t, err)
	got := hasNext(bytes.NewReader(b))
	require.False(t, got)
}

func TestPageWithURL(t *testing.T) {
	tests := map[string]struct {
		url     string
		want    int
		wantErr string
	}{
		"no-trailing-slash": {
			url:  "https://example.com/page/2",
			want: 2,
		},
		"trailing-slash": {
			url:  "https://example.com/page/2/",
			want: 2,
		},
		"bad-url": {
			url:     ";",
			wantErr: "strconv.Atoi: parsing \";\": invalid syntax",
		},
	}
	for desc, tt := range tests {
		got, err := pageWithURL(tt.url)
		if tt.wantErr == "" {
			require.NoError(t, err, desc)
			require.Greater(t, got, 0, desc)
		} else {
			require.Error(t, err, desc)
			require.EqualError(t, err, "strconv.Atoi: parsing \";\": invalid syntax", desc)
		}
	}
}

func TestParseDivPaginationNext(t *testing.T) {
	tests := map[string]struct {
		pagination *Pagination
		html       string
		want       *Pagination
	}{
		"normal": {
			pagination: &Pagination{},
			html:       `<div class="pagination"> <div class="paginate-nextprev paginate-disabled"><span class="previous">Previous</span></div> <div class="paginate-nextprev"><a class="next" href="/alright__fine/followers/page/2/">Next</a></div> </div>`,
			want: &Pagination{
				CurrentPage: 1,
				NextPage:    2,
			},
		},
		"missing-href": {
			pagination: &Pagination{},
			html:       `<div class="pagination"> <div class="paginate-nextprev paginate-disabled"><span class="previous">Previous</span></div> <div class="paginate-nextprev"><a class="next">Next</a></div> </div>`,
			want:       &Pagination{},
		},
		"no-page-in-href": {
			pagination: &Pagination{},
			html:       `<div class="pagination"> <div class="paginate-nextprev paginate-disabled"><span class="previous">Previous</span></div> <div class="paginate-nextprev"><a class="next" href="/alright__fine/followers/">Next</a></div> </div>`,
			want:       &Pagination{},
		},
	}
	for dest, tt := range tests {
		sel := selectWithString(tt.html)
		tt.pagination.parseDivPaginationNext(sel)
		require.Equal(t, tt.want, tt.pagination, dest)
	}
}

func TestParseDivPaginationPrevious(t *testing.T) {
	tests := map[string]struct {
		pagination *Pagination
		html       string
		want       *Pagination
	}{
		"normal": {
			pagination: &Pagination{},
			html:       `<div class="pagination"> <div class="paginate-nextprev"><a class="previous" href="/films/popular/page/2/">Previous</a></div> <div class="paginate-nextprev"><a class="next" href="/films/popular/page/4/">Next</a></div> </div>`,
			want: &Pagination{
				CurrentPage: 3,
			},
		},
		"missing-href": {
			pagination: &Pagination{},
			html:       `<div class="pagination"> <div class="paginate-nextprev"><a class="previous">Previous</a></div> <div class="paginate-nextprev"><a class="next" href="/films/popular/page/4/">Next</a></div> </div>`,
			want:       &Pagination{},
		},
		"no-page-in-href": {
			pagination: &Pagination{},
			html:       `<div class="pagination"> <div class="paginate-nextprev"><a class="previous" href="/films/popular/">Previous</a></div> <div class="paginate-nextprev"><a class="next" href="/films/popular/page/4/">Next</a></div> </div>`,
			want:       &Pagination{},
		},
	}
	for dest, tt := range tests {
		sel := selectWithString(tt.html)
		tt.pagination.parseDivPaginationPrevious(sel)
		require.Equal(t, tt.want, tt.pagination, dest)
	}
}

// selectWithString takes a string, and converts it in to a goquery.Selection. Really just useful for helping test
func selectWithString(s string) *goquery.Selection {
	doc := mustNewDocumentFromReader(strings.NewReader(s))
	return doc.Find("*")
}
