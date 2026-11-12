package crproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestCheckAndApplyWhitelistsAndBlacklists(t *testing.T) {
	whitelist := newReposList([]string{"library/*"})
	blacklist := newReposList([]string{"library/secret"})

	w := httptest.NewRecorder()
	require.True(t, checkAndApplyWhitelists(whitelist, blacklist, w, []string{"library", "ubuntu"}))

	w = httptest.NewRecorder()
	require.False(t, checkAndApplyWhitelists(whitelist, blacklist, w, []string{"other", "ubuntu"}))
	require.Equal(t, http.StatusForbidden, w.Code)

	w = httptest.NewRecorder()
	require.False(t, checkAndApplyWhitelists(whitelist, blacklist, w, []string{"library", "secret"}))
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestExtractReposNameFromV1Path(t *testing.T) {
	for _, tc := range []struct {
		path string
		want []string // nil if not recognized
	}{
		{"/v1/repositories/foo/images", []string{"foo"}},
		{"/v1/repositories/foo/tags", []string{"foo"}},
		{"/v1/repositories/foo/tags/latest", []string{"foo"}},
		{"/v1/repositories/someone/somerepos/images", []string{"someone", "somerepos"}},
		{"/v1/repositories/someone/somerepos/tags/latest", []string{"someone", "somerepos"}},
		{"/v2/foo/images", nil},
		{"/v1/repositories/", nil},
		{"/v1/repositories/foo", nil},
		{"/v1/repositories/foo/tags/", nil},
		{"/v1/repositories/foo/tags/nested/path", nil},
	} {
		got := extractReposNameFromV1Path(tc.path)
		if tc.want == nil {
			require.Nil(t, got, tc.path)
		} else {
			require.NotNil(t, got, tc.path)
			require.Equal(t, tc.want, *got, tc.path)
		}
	}
}

func TestExtractReposNameFromV2Path(t *testing.T) {
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"/v2/foo/manifests/latest", []string{"foo"}},
		{"/v2/someone/somerepos/blobs/sha256:abc123", []string{"someone", "somerepos"}},
		{"/v2/foo/bar/baz/tags/list", []string{"foo", "bar", "baz"}},
		{"/v2/org/blobs/uploads/", []string{"org"}},
		{"/v2/org/blobs/uploads/session.id_a-=", []string{"org"}},
		// Repository segments may have the same names as API operations
		{"/v2/org/blobs/uploads/manifests/latest", []string{"org", "blobs", "uploads"}},
		{"/v2/org/tags/list/tags/list", []string{"org", "tags", "list"}},
		{"/v2/org/manifests/x/blobs/uploads/id", []string{"org", "manifests", "x"}},
		// Operation keywords are also valid tags and opaque upload IDs
		{"/v2/org/manifests/blobs", []string{"org"}},
		{"/v2/org/blobs/uploads/uploads", []string{"org"}},
		// A registry redirect must not let trailing-slash variants evade repository checks
		{"/v2/org/manifests/latest/", []string{"org"}},
		{"/v2/org/tags/list//", []string{"org"}},
		{"/v2/org/blobs/uploads", []string{"org"}},
	} {
		got, ok := extractReposNameFromV2Path(tc.path)
		require.True(t, ok, tc.path)
		require.Equal(t, tc.want, got, tc.path)
	}

	for _, path := range []string{"/v2/", "/v2/_catalog"} {
		got, ok := extractReposNameFromV2Path(path)
		require.True(t, ok, path)
		require.Nil(t, got, path)
	}

	for _, path := range []string{
		"",
		"/v20/org/image/manifests/latest",
		"/v2/org/image",
		"/v2/org/image/manifests",
		"/v2/org/image/tags/list/extra",
		"/v2/org/image/referrers/sha256:abc123",
		"/v2/manifests/latest",
		"/v2/blobs/uploads/",
		"/v2/_catalog/",
		"/v2/org//image/manifests/latest",
		"/v2/open/../org/image/manifests/latest",
		"/v2/org/image/manifests/latest/.",
	} {
		got, ok := extractReposNameFromV2Path(path)
		require.False(t, ok, path)
		require.Nil(t, got, path)
	}
}

func TestSingleProxyCheckReposWhitelist(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		whitelist, blacklist []string
		path                 string
		route                routePrefix
		want                 bool
	}{
		{"no lists, unrecognized path", nil, nil, "/v2/org/image/referrers/sha256:abc123", routePrefixV2, true},
		{"whitelist, unrecognized path", []string{"org"}, nil, "/v2/org//image/manifests/latest", routePrefixV2, false},
		{"blacklist, unrecognized path", nil, []string{"other"}, "/v2/org/image/referrers/sha256:abc123", routePrefixV2, false},
		{"registry-wide endpoint", []string{"org"}, nil, "/v2/_catalog", routePrefixV2, true},
		{"v1 repository", []string{"org"}, nil, "/v1/repositories/other/image/tags", routePrefixV1, false},
		{"v1 without repository", []string{"org"}, nil, "/v1/search", routePrefixV1, true},
	} {
		h := &singleProxyHandler{whitelist: newReposList(tc.whitelist), blacklist: newReposList(tc.blacklist)}
		w := httptest.NewRecorder()
		require.Equal(t, tc.want, h.checkReposWhitelist(testutils.Context(), w, tc.path, tc.route), tc.name)
		if !tc.want {
			require.Equal(t, http.StatusForbidden, w.Code, tc.name)
		}
	}
}
