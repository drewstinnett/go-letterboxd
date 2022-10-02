package letterboxd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetOfficialListMap(t *testing.T) {
	got := sc.List.GetOfficialMap(context.TODO())
	require.NotNil(t, got)
	require.Greater(t, len(got), 0)
}
