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

func TestParseTargetUrlDefaultsSchemeForHostOnlyPath(t *testing.T) {
	h := &proxyHandler{}
	w := httptest.NewRecorder()

	target, ok := h.parseTargetUrl(w, "/github.com/owner/repo.git/info/refs")
	require.True(t, ok)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "https", target.Scheme)
	require.Equal(t, "github.com", target.Host)
	require.Equal(t, "/owner/repo.git/info/refs", target.Path)
}

func TestParseTargetUrlPreservesExplicitScheme(t *testing.T) {
	h := &proxyHandler{}
	w := httptest.NewRecorder()

	target, ok := h.parseTargetUrl(w, "/https://github.com/owner/repo.git/info/refs")
	require.True(t, ok)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "https", target.Scheme)
	require.Equal(t, "github.com", target.Host)
	require.Equal(t, "/owner/repo.git/info/refs", target.Path)
}

func TestParseTargetUrlDoesNotTreatPathDelimiterAsScheme(t *testing.T) {
	h := &proxyHandler{}
	w := httptest.NewRecorder()

	target, ok := h.parseTargetUrl(w, "/github.com/owner/repo/raw/main/file://name")
	require.True(t, ok)
	require.Equal(t, "https", target.Scheme)
	require.Equal(t, "github.com", target.Host)
	require.Equal(t, "/owner/repo/raw/main/file://name", target.Path)
}

func TestParseTargetUrlRejectsUnsupportedForms(t *testing.T) {
	for _, reqPath := range []string{
		"///github.com/owner/repo.git/info/refs",
		"/https:github.com/owner/repo.git/info/refs",
		"/http://github.com/owner/repo.git/info/refs",
		"/github.com:443/owner/repo.git/info/refs",
		"/user:pass@github.com/owner/repo.git/info/refs",
	} {
		t.Run(reqPath, func(t *testing.T) {
			h := &proxyHandler{}
			w := httptest.NewRecorder()

			_, ok := h.parseTargetUrl(w, reqPath)
			require.False(t, ok)
			require.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestServeHttpRewritesRawTextURLs(t *testing.T) {
	setAllowInsecureTargetForTest(true)
	defer setAllowInsecureTargetForTest(false)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("see https://" + r.Host + "/org/repo/blob"))
	}))
	defer upstream.Close()
	parsed := mustParseURL(upstream.URL)
	allowedHosts[parsed.Host] = hostDefinition2(githubRawPathPattern)
	rawTextUrlRewriteHosts[parsed.Host] = true
	defer delete(allowedHosts, parsed.Host)
	defer delete(rawTextUrlRewriteHosts, parsed.Host)

	info := testutils.SiteInfo("gh", config.SiteModeGithubDownloadProxy, "/gh", "http://proxy")
	settings := &config.GithubDownloadProxySettings{RawTextUrlRewrite: true}
	h, err := NewGithubProxyHandler(info, testutils.NewRequestHelper(t), settings)
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodGet, "/gh/"+upstream.URL+"/org/repo/main/file", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "http://proxy/https://"+parsed.Host+"/")
}

func TestServeHttpRejectsKnownLengthResponseOverLimit(t *testing.T) {
	setAllowInsecureTargetForTest(true)
	defer setAllowInsecureTargetForTest(false)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer upstream.Close()
	parsed := mustParseURL(upstream.URL)
	allowedHosts[parsed.Host] = hostDefinition2(githubMainPathPattern)
	defer delete(allowedHosts, parsed.Host)

	info := testutils.SiteInfo("gh", config.SiteModeGithubDownloadProxy, "/gh", "")
	limit := int64(4)
	h, err := NewGithubProxyHandler(info, testutils.NewRequestHelper(t), &config.GithubDownloadProxySettings{SizeLimit: limit})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/gh/"+upstream.URL+"/org/repo/raw/main/file", nil))
	require.Equal(t, http.StatusBadGateway, w.Code)
}

func TestServeHttpDoesNotRewriteNonTextResponse(t *testing.T) {
	setAllowInsecureTargetForTest(true)
	defer setAllowInsecureTargetForTest(false)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("https://" + r.Host + "/org/repo/blob"))
	}))
	defer upstream.Close()
	parsed := mustParseURL(upstream.URL)
	allowedHosts[parsed.Host] = hostDefinition2(githubRawPathPattern)
	rawTextUrlRewriteHosts[parsed.Host] = true
	defer delete(allowedHosts, parsed.Host)
	defer delete(rawTextUrlRewriteHosts, parsed.Host)

	h, err := NewGithubProxyHandler(testutils.SiteInfo("gh", config.SiteModeGithubDownloadProxy, "/gh", "http://proxy"), testutils.NewRequestHelper(t), &config.GithubDownloadProxySettings{RawTextUrlRewrite: true})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/gh/"+upstream.URL+"/org/repo/main/file", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "https://"+parsed.Host+"/org/repo/blob")
}
