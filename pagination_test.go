package letterboxd

import (
	"os"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
)

func TestExtractPagination(t *testing.T) {
	var pagination *Pagination
	f, err := os.Open("testdata/user/films.html")
	defer f.Close()
	require.NoError(t, err)
	doc, err := goquery.NewDocumentFromReader(f)
	require.NoError(t, err)

	pagination, err = ExtractPaginationWithDoc(doc)
	require.NoError(t, err)
	require.Equal(t, 1, pagination.CurrentPage)
	require.Equal(t, 59, pagination.TotalPages)
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
		pagination, err := ExtractPaginationWithBytes(tt.content)
		require.Equal(t, tt.expectedError, err)
		require.Equal(t, tt.expectedPagination, pagination)
	}
}
