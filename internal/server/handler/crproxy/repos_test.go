package crproxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/stretchr/testify/assert"
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
	tests := []struct {
		name     string
		path     string
		expected *[]string
	}{
		// Single segment repository name
		{
			name:     "Single segment - images",
			path:     "/v1/repositories/foo/images",
			expected: utils.ToPtr([]string{"foo"}),
		},
		{
			name:     "Single segment - tags",
			path:     "/v1/repositories/foo/tags",
			expected: utils.ToPtr([]string{"foo"}),
		},
		{
			name:     "Single segment - specific tag",
			path:     "/v1/repositories/foo/tags/latest",
			expected: utils.ToPtr([]string{"foo"}),
		},
		// Two segments repository name (namespace/name)
		{
			name:     "Two segments - images",
			path:     "/v1/repositories/someone/somerepos/images",
			expected: utils.ToPtr([]string{"someone", "somerepos"}),
		},
		{
			name:     "Two segments - tags",
			path:     "/v1/repositories/someone/somerepos/tags",
			expected: utils.ToPtr([]string{"someone", "somerepos"}),
		},
		{
			name:     "Two segments - specific tag",
			path:     "/v1/repositories/someone/somerepos/tags/latest",
			expected: utils.ToPtr([]string{"someone", "somerepos"}),
		},
		// Invalid paths
		{
			name:     "Invalid path - no v1 prefix",
			path:     "/v2/foo/images",
			expected: nil,
		},
		{
			name:     "Invalid path - empty path",
			path:     "/v1/repositories/",
			expected: nil,
		},
		{
			name:     "Invalid path - no suffix",
			path:     "/v1/repositories/foo",
			expected: nil,
		},
		{
			name:     "Invalid path - tags with no tag name",
			path:     "/v1/repositories/foo/tags/",
			expected: nil,
		},
		{
			name:     "Invalid path - tags with nested path",
			path:     "/v1/repositories/foo/tags/nested/path",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractReposNameFromV1Path(tt.path)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestExtractReposNameFromV2Path(t *testing.T) {
	type testCase struct {
		name     string
		path     string
		expected *[]string
		ok       bool
	}
	tests := []testCase{
		{
			name:     "Single segment - tags list",
			path:     "/v2/foo/tags/list",
			expected: utils.ToPtr([]string{"foo"}),
			ok:       true,
		},
		{
			name:     "Single segment - manifests",
			path:     "/v2/foo/manifests/latest",
			expected: utils.ToPtr([]string{"foo"}),
			ok:       true,
		},
		{
			name:     "Single segment - blobs",
			path:     "/v2/foo/blobs/sha256:abc123",
			expected: utils.ToPtr([]string{"foo"}),
			ok:       true,
		},
		{
			name:     "Two segments - tags list",
			path:     "/v2/someone/somerepos/tags/list",
			expected: utils.ToPtr([]string{"someone", "somerepos"}),
			ok:       true,
		},
		{
			name:     "Two segments - manifests",
			path:     "/v2/someone/somerepos/manifests/latest",
			expected: utils.ToPtr([]string{"someone", "somerepos"}),
			ok:       true,
		},
		{
			name:     "Two segments - blobs",
			path:     "/v2/someone/somerepos/blobs/sha256:abc123",
			expected: utils.ToPtr([]string{"someone", "somerepos"}),
			ok:       true,
		},
		{
			name:     "Three segments - tags list",
			path:     "/v2/foo/bar/baz/tags/list",
			expected: utils.ToPtr([]string{"foo", "bar", "baz"}),
			ok:       true,
		},
		{
			name:     "Three segments - manifests",
			path:     "/v2/foo/bar/baz/manifests/latest",
			expected: utils.ToPtr([]string{"foo", "bar", "baz"}),
			ok:       true,
		},
		{
			name:     "Three segments - blobs",
			path:     "/v2/foo/bar/baz/blobs/sha256:abc123",
			expected: utils.ToPtr([]string{"foo", "bar", "baz"}),
			ok:       true,
		},
		{
			name:     "Invalid path - no v2 prefix",
			path:     "/v1/foo/bar/baz/tags/list",
			expected: nil,
		},
		{
			name:     "Catalog path",
			path:     "/v2/_catalog",
			expected: nil,
			ok:       true,
		},
	}

	tests = append(tests, testCase{name: "Registry root", path: "/v2/", ok: true})

	// Repository segments may have the same names as API operations.
	for _, repo := range []string{
		"org/blobs/blocked",
		"org/manifests/blocked",
		"org/tags/list/private",
		"org/blobs/uploads/private",
		"org/blobs",
		"org/blobs/uploads",
		"org/tags/list",
		"blobs/manifests/tags/list/uploads",
	} {
		for _, suffix := range []string{
			"/manifests/latest",
			"/blobs/sha256:abc123",
			"/tags/list",
			"/blobs/uploads/",
			"/blobs/uploads/session.id_a-=",
		} {
			tests = append(tests, testCase{
				name: repo + suffix, path: "/v2/" + repo + suffix,
				expected: utils.ToPtr(strings.Split(repo, "/")),
				ok:       true,
			})
		}
	}
	// Operation keywords are also valid tags and opaque upload IDs.
	for _, suffix := range []string{
		"/manifests/blobs",
		"/manifests/manifests",
		"/manifests/tags",
		"/manifests/list",
		"/manifests/uploads",
		"/manifests/sha512:" + strings.Repeat("a", 128),
		"/blobs/uploads/manifests",
		"/blobs/uploads/list",
		"/blobs/uploads/uploads",
	} {
		tests = append(tests, testCase{
			name: "Keyword parameter " + suffix, path: "/v2/org" + suffix,
			expected: utils.ToPtr([]string{"org"}),
			ok:       true,
		})
	}
	// A registry redirect must not let trailing-slash variants evade repository checks.
	for _, suffix := range []string{
		"/manifests/latest/",
		"/manifests/latest//",
		"/blobs/sha256:abc123/",
		"/tags/list/",
		"/blobs/uploads",
		"/blobs/uploads//",
		"/blobs/uploads/id/",
	} {
		tests = append(tests, testCase{
			name: "Trailing slash variant " + suffix, path: "/v2/org/blobs/uploads" + suffix,
			expected: utils.ToPtr([]string{"org", "blobs", "uploads"}),
			ok:       true,
		})
	}
	for _, path := range []string{
		"",
		"/",
		"/v20/org/image/manifests/latest",
		"/v2/org/image",
		"/v2/org/image/manifests",
		"/v2/org/image/manifests/",
		"/v2/org/image/blobs/",
		"/v2/org/image/tags",
		"/v2/org/image/tags/other",
		"/v2/org/image/tags/list/extra",
		"/v2/org/image/blobs/uploads/id/extra",
		"/v2/manifests/latest",
		"/v2/tags/list",
		"/v2/blobs/uploads/",
		"/v2//manifests/latest",
		"/v2//blobs/uploads/id",
		"/v2/org//image/manifests/latest",
		"/v2/org/image/manifests/latest/.",
		"/v2/open/../org/image/manifests/latest",
		"/v2/org/./image/manifests/latest",
		"/v2/org/image/blobs/uploads/..",
		"/v2/org/image/referrers/sha256:abc123",
		"/v2/_catalog/",
	} {
		tests = append(tests, testCase{name: "Invalid path " + path, path: path})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := extractReposNameFromV2Path(tt.path)
			require.Equal(t, tt.ok, ok)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tt.expected, result)
			}
		})
	}
}

func TestSingleProxyReposListsHandleUnrecognizedPaths(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		whitelist, blacklist []string
		unknownStatus        int
	}{
		{"no lists", nil, nil, http.StatusOK},
		{"whitelist", []string{"org"}, nil, http.StatusForbidden},
		{"blacklist", nil, []string{"org"}, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int64
			mux := http.NewServeMux()
			mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				mux.ServeHTTP(w, r)
			}))
			t.Cleanup(upstream.Close)
			info := testutils.SiteInfo("repos", config.SiteModeContainerRegistrySingleProxy, "/r", "http://proxy.test")
			h, err := NewContainerRegistrySingleProxyHandler(info, testutils.NewRequestHelper(t), &config.ContainerRegistrySingleProxySettings{
				UpstreamV2Url:  strPtr(upstream.URL + "/v2"),
				Auth:           testAuthConfig(false),
				AllowPush:      boolPtr(false),
				AllowList:      boolPtr(true),
				ReposWhitelist: tc.whitelist,
				ReposBlacklist: tc.blacklist,
			})
			require.NoError(t, err)
			t.Cleanup(h.Shutdown)
			for _, path := range []string{
				"/v2/org//image/manifests/latest",
				"/v2/org/image/manifests/latest/.",
				"/v2/open/../org/image/manifests/latest",
				"/v2/org/image/referrers/sha256:abc123",
			} {
				t.Run(path, func(t *testing.T) {
					before := hits.Load()
					w := serveRequest(h, newRequest(http.MethodGet, "/r"+path, ""))
					require.Equal(t, tc.unknownStatus, w.Code)
					if tc.unknownStatus == http.StatusForbidden {
						require.Equal(t, before, hits.Load(), "unrecognized paths must be denied before contacting the upstream")
					} else {
						require.Greater(t, hits.Load(), before, "without lists, upstream extensions and path redirects remain usable")
					}
				})
			}
			for _, path := range []string{
				"/v2/",
				"/v2/_catalog",
			} {
				w := serveRequest(h, newRequest(http.MethodGet, "/r"+path, ""))
				require.Equal(t, http.StatusOK, w.Code, "registry-wide endpoints are not repository whitelist entries: "+path)
			}
		})
	}
}
