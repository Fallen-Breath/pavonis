package pypiproxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

// fakeIndex serves https://pypi.test, and records the url of each request
type fakeIndex struct {
	requests []string
	handle   func(w http.ResponseWriter, r *http.Request)
}

func (f *fakeIndex) RoundTrip(req *http.Request) (*http.Response, error) {
	f.requests = append(f.requests, req.URL.String())
	w := httptest.NewRecorder()
	f.handle(w, req)
	resp := w.Result()
	resp.Request = req
	return resp, nil
}

func newTestHandler(t *testing.T, index *fakeIndex, filesUrl string) handler.HttpHandler {
	t.Helper()
	info := testutils.SiteInfo("pypi", config.SiteModePypiProxy, "/pypi", "")
	h, err := NewProxyHandler(info, testutils.NewRequestHelperWithTransport(t, index), &config.PypiRegistrySettings{
		UpstreamSimpleUrl: configStringPtr("https://pypi.test/simple"),
		UpstreamFilesUrl:  configStringPtr(filesUrl),
	})
	require.NoError(t, err)
	return h
}

func serveIndex(t *testing.T, contentType, body, filesUrl, path string) (*fakeIndex, *httptest.ResponseRecorder) {
	t.Helper()
	index := &fakeIndex{handle: func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(body))
	}}
	w := httptest.NewRecorder()
	newTestHandler(t, index, filesUrl).ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, w.Code)
	return index, w
}

func TestServeHttpReturnsNotFoundForUnknownPath(t *testing.T) {
	index := &fakeIndex{}
	w := httptest.NewRecorder()
	newTestHandler(t, index, "https://pypi.test/files").ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/pypi/other", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Empty(t, index.requests)
}

func TestServeHttpRewritesSimpleProjectListLinks(t *testing.T) {
	index, w := serveIndex(t, "text/html; charset=utf-8", `<a href="/simple/demo/">demo</a><a href="other/">other</a>`, "https://pypi.test/files", "/pypi/simple/")
	require.Equal(t, []string{"https://pypi.test/simple/"}, index.requests)
	require.Equal(t, `<a href="/pypi/simple/demo/">demo</a><a href="other/">other</a>`, w.Body.String())
}

func TestServeHttpRewritesSimpleProjectLinks(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, body, expected string
	}{
		{"html", "text/html", `<a href="https://pypi.test/files/demo.whl">pkg</a>`, `<a href="/pypi/files/demo.whl">pkg</a>`},
		{"json", "application/vnd.pypi.simple.v1+json", `{"files":[{"url":"https://pypi.test/files/demo.whl"}]}`, `{"files":[{"url":"/pypi/files/demo.whl"}]}`},
		{"spaced json", "application/vnd.pypi.simple.v1+json", `{"files":[{"url": "https://pypi.test/files/demo.whl"}]}`, `{"files":[{"url": "/pypi/files/demo.whl"}]}`},
		{"not a simple index", "application/json", `{"url":"https://pypi.test/files/demo.whl"}`, `{"url":"https://pypi.test/files/demo.whl"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index, w := serveIndex(t, tc.contentType, tc.body, "https://pypi.test/files", "/pypi/simple/demo/")
			require.Equal(t, []string{"https://pypi.test/simple/demo/"}, index.requests)
			require.Equal(t, tc.expected, w.Body.String())
		})
	}
}

func TestServeHttpRewritesProjectPageWithoutTrailingSlash(t *testing.T) {
	_, w := serveIndex(t, "text/html", `<a href="https://pypi.test/files/demo.whl">pkg</a>`, "https://pypi.test/files", "/pypi/simple/demo")
	require.Equal(t, `<a href="/pypi/files/demo.whl">pkg</a>`, w.Body.String())
}

func TestServeHttpLeavesUnmatchedLinksUnchanged(t *testing.T) {
	const body = `<a href="https://other.example/files/a.whl">a</a><a href="/files-other/b.whl">b</a><a href="//other.example/c.whl">c</a>`
	for _, filesUrl := range []string{"https://pypi.test/files", "https://pypi.test"} {
		_, w := serveIndex(t, "text/html", body, filesUrl, "/pypi/simple/demo/")
		require.Equal(t, body, w.Body.String(), filesUrl)
	}
}

func TestServeHttpRewritesLinksRelativeToFinalIndexUrl(t *testing.T) {
	index := &fakeIndex{handle: func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/simple/demo/" {
			http.Redirect(w, r, "/nested/simple/demo/", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<a href="../../packages/ab/pkg.whl#sha256=abc">pkg</a>`))
	}}
	w := httptest.NewRecorder()
	newTestHandler(t, index, "https://pypi.test/nested/packages").ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/pypi/simple/demo/", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, `<a href="/pypi/files/ab/pkg.whl#sha256=abc">pkg</a>`, w.Body.String())
}

func TestServeHttpForwardsEscapedFilePath(t *testing.T) {
	index := &fakeIndex{handle: func(w http.ResponseWriter, r *http.Request) {}}
	w := httptest.NewRecorder()
	newTestHandler(t, index, "https://pypi.test/packages").ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/pypi/files/ab/a%2Fb%2Bc.whl?download=1", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []string{"https://pypi.test/packages/ab/a%2Fb%2Bc.whl?download=1"}, index.requests)
}

func TestSimpleFileUrlPrefixes(t *testing.T) {
	for _, tc := range []struct {
		name, base, files string
		expected          []string
	}{
		{"sibling directories", "https://index.example/simple/demo/", "https://index.example/packages", []string{
			"https://index.example/packages/",
			"/packages/",
			"../../packages/",
		}},
		{"nested mirror directories", "https://index.example/pypi/web/simple/demo/", "https://index.example/pypi/web/packages", []string{
			"https://index.example/pypi/web/packages/",
			"/pypi/web/packages/",
			"../../packages/",
		}},
		{"index without trailing slash", "https://index.example/simple/demo", "https://index.example/packages", []string{
			"https://index.example/packages/",
			"/packages/",
			"../packages/",
		}},
		{"different file host", "https://index.example/simple/demo/", "https://files.example/packages", []string{
			"https://files.example/packages/",
		}},
		{"different scheme", "http://index.example/simple/demo/", "https://index.example/packages", []string{
			"https://index.example/packages/",
		}},
		{"files at root", "https://index.example/demo/", "https://index.example", []string{
			"https://index.example/",
			"../",
		}},
		{"index at root", "https://index.example/", "https://index.example", []string{
			"https://index.example/",
			"./",
		}},
		{"shared file and index directory", "https://index.example/packages/demo/", "https://index.example/packages/demo", []string{
			"https://index.example/packages/demo/",
			"/packages/demo/",
			"./",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, err := url.Parse(tc.base)
			require.NoError(t, err)
			files, err := url.Parse(tc.files)
			require.NoError(t, err)
			require.Equal(t, tc.expected, simpleFileUrlPrefixes(base, files))
		})
	}
}

func configStringPtr(value string) *string { return &value }
