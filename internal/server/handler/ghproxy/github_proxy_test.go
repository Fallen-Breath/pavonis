package ghproxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func TestServeHttpForwardsGithubRequest(t *testing.T) {
	setAllowInsecureTargetForTest(true)
	defer setAllowInsecureTargetForTest(false)
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("github body"))
	}))
	defer upstream.Close()
	parsed := mustParseURL(upstream.URL)
	allowedHosts[parsed.Host] = hostDefinition2(githubMainPathPattern)
	defer delete(allowedHosts, parsed.Host)
	info := testutils.SiteInfo("gh", config.SiteModeGithubDownloadProxy, "/gh", "")
	h, err := NewGithubProxyHandler(info, testutils.NewRequestHelper(t), &config.GithubDownloadProxySettings{})
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodGet, "/gh/"+upstream.URL+"/org/repo/raw/main/file.txt", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "/org/repo/raw/main/file.txt", gotPath)
	require.Equal(t, "github body", w.Body.String())
}

func TestServeHttpRejectsGithubForbiddenHost(t *testing.T) {
	info := testutils.SiteInfo("gh", config.SiteModeGithubDownloadProxy, "/gh", "")
	h, err := NewGithubProxyHandler(info, testutils.NewRequestHelper(t), &config.GithubDownloadProxySettings{})
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodGet, "/gh/https://example.com/org/repo/raw/main/file.txt", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)
	require.Equal(t, http.StatusNotFound, w.Code)
}
