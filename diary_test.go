package letterboxd

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractUserDiary(t *testing.T) {
	data, err := os.ReadFile("testdata/user/diary-paginated/1.html")
	require.NoError(t, err)

	itemsI, _, err := sc.User.ExtractDiaryEntries(bytes.NewReader(data))
	items := itemsI.([]*DiaryEntry)
	require.NoError(t, err)
	require.Equal(t, len(items), 50)
	require.Equal(t, 7, *items[0].Rating)
	require.Equal(t, "cure", *items[0].Slug)
	require.Equal(t, true, *&items[0].SpecifiedDate)
	require.Equal(t, true, *&items[0].Rewatch)

	require.NotNil(t, items[0].Film)
	require.Equal(t, "Sweet Sweetback's Baadasssss Song", items[0].Film.Title)
}
