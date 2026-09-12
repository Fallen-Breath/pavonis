package crproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractReposNameFromPaths(t *testing.T) {
	for _, tc := range []struct {
		path string
		v2   bool
		want []string
	}{
		{"/v1/repositories/library/ubuntu/tags", false, []string{"library", "ubuntu"}},
		{"/v1/repositories/ubuntu/tags/latest", false, []string{"ubuntu"}},
		{"/v2/library/ubuntu/manifests/latest", true, []string{"library", "ubuntu"}},
		{"/v2/library/ubuntu/blobs/uploads/abc", true, []string{"library", "ubuntu"}},
	} {
		got := extractReposNameFromV1Path(tc.path)
		if tc.v2 {
			got = extractReposNameFromV2Path(tc.path)
		}
		require.NotNil(t, got, tc.path)
		require.Equal(t, tc.want, *got, tc.path)
	}
	for _, path := range []string{"/v1/nope", "/v1/repositories/a/tags/b/c", "/v2/_catalog"} {
		require.Nil(t, extractReposNameFromV1Path(path))
	}
}

func TestSplitAnyProxyPath(t *testing.T) {
	for _, tc := range []struct {
		path, host, routed string
	}{
		{"/registry.example/v2/", "registry.example", "/v2/"},
		{"/registry.example", "registry.example", "/"},
		{"/registry.example/auth", "registry.example", "/auth"},
	} {
		host, routed, ok := splitAnyProxyPath(tc.path)
		require.True(t, ok)
		require.Equal(t, tc.host, host)
		require.Equal(t, tc.routed, routed)
	}
	for _, path := range []string{"", "registry.example", "/"} {
		_, _, ok := splitAnyProxyPath(path)
		require.False(t, ok)
	}
}
