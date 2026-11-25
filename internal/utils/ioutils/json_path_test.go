package ioutils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseJsonPath(t *testing.T) {
	fileUrls := []JsonPathSegment{NewJsonPathSegmentKey("files"), {kind: jsonPathSegmentWildcard}, NewJsonPathSegmentKey("url")}
	for _, tc := range []struct {
		path string
		want []JsonPathSegment
	}{
		{"files[*].url", fileUrls},
		{"$.files.*.url", fileUrls},
		{`meta["*"]`, []JsonPathSegment{NewJsonPathSegmentKey("meta"), NewJsonPathSegmentKey("*")}},
		{`$['it\'s "ok"']`, []JsonPathSegment{NewJsonPathSegmentKey(`it's "ok"`)}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got, err := ParseJsonPath(tc.path)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
	root, err := ParseJsonPath("$")
	require.NoError(t, err)
	require.Empty(t, root)
}

func TestParseJsonPathRejectsInvalidPaths(t *testing.T) {
	for _, path := range []string{"", "$a", "a..b", "files[*"} {
		_, err := ParseJsonPath(path)
		require.Error(t, err, "path %q", path)
	}
}
