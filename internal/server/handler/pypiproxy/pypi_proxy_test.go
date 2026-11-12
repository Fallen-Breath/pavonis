package pypiproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestServeHttpRewritesSimpleHtmlLinks(t *testing.T) {
	var upstreamURL string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/simple/demo", r.URL.Path)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fmt.Sprintf(`<a href="%s/files/pkg.whl">pkg</a>`, upstreamURL)))
	}))
	defer upstream.Close()
	upstreamURL = upstream.URL

	h := newPypiTestHandler(t, upstream.URL)
	r := httptest.NewRequest(http.MethodGet, "/pypi/simple/demo", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `href="/pypi/files/pkg.whl"`)
}

func TestServeHttpReturnsNotFoundForUnknownPath(t *testing.T) {
	h := newPypiTestHandler(t, "http://127.0.0.1")
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/pypi/other", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestServeHttpRewritesSimpleProjectListLinks(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/simple/", r.URL.Path)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<a href="/simple/demo/">demo</a><a href="other/">other</a>`))
	}))
	defer upstream.Close()

	h := newPypiTestHandler(t, upstream.URL)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/pypi/simple/", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, `<a href="/pypi/simple/demo/">demo</a><a href="other/">other</a>`, w.Body.String())
}

func TestServeHttpRewritesSimpleProjectJSONLinks(t *testing.T) {
	var upstreamURL string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/simple/demo/", r.URL.Path)
		w.Header().Set("Content-Type", "application/vnd.pypi.simple.v1+json")
		_, _ = fmt.Fprintf(w, `{"files":[{"url":%s"%s/files/demo.whl"}]}`, r.URL.Query().Get("space"), upstreamURL)
	}))
	defer upstream.Close()
	upstreamURL = upstream.URL

	h := newPypiTestHandler(t, upstream.URL)
	for _, space := range []string{"", " "} {
		w := httptest.NewRecorder()
		h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/pypi/simple/demo/?space="+url.QueryEscape(space), nil))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, `{"files":[{"url":`+space+`"/pypi/files/demo.whl"}]}`, w.Body.String())
	}
}

func TestServeHttpRelativePackageDownloads(t *testing.T) {
	for _, tc := range []struct {
		name, simplePath, filesPath, link, fileSuffix string
		json, spaced, redirect                        bool
	}{
		{"parent relative HTML", "/simple", "/packages", "../../packages/ab/pkg.whl", "/ab/pkg.whl", false, false, false},
		{"parent relative JSON", "/simple", "/packages", "../../packages/ab/pkg.whl", "/ab/pkg.whl", true, false, false},
		{"spaced JSON with nested upstream paths", "/pypi/web/simple", "/pypi/web/packages", "../../packages/ab/pkg.whl", "/ab/pkg.whl", true, true, false},
		{"root relative HTML", "/simple", "/packages", "/packages/ab/pkg.whl", "/ab/pkg.whl", false, false, false},
		{"same directory filename", "", "", "pkg.whl", "/demo/pkg.whl", false, false, false},
		{"relative to final index after redirect", "/simple", "/nested/packages", "../../packages/ab/pkg.whl", "/ab/pkg.whl", false, false, true},
		{"escaped file path", "/simple", "/packages", "../../packages/ab/a%2Fb%2Bc.whl", "/ab/a%2Fb%2Bc.whl", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.EscapedPath() == tc.filesPath+tc.fileSuffix {
					require.Equal(t, "download=1", r.URL.RawQuery)
					_, _ = w.Write([]byte("wheel contents"))
					return
				}
				if tc.redirect && r.URL.Path == tc.simplePath+"/demo/" {
					http.Redirect(w, r, "/nested/simple/demo/", http.StatusFound)
					return
				}
				indexPath := tc.simplePath + "/demo/"
				if tc.redirect {
					indexPath = "/nested/simple/demo/"
				}
				require.Equal(t, indexPath, r.URL.Path)
				link := tc.link + "?download=1#sha256=abc"
				if tc.json {
					w.Header().Set("Content-Type", "application/vnd.pypi.simple.v1+json")
					space := ""
					if tc.spaced {
						space = " "
					}
					_, _ = fmt.Fprintf(w, `{"files":[{"url":%s%q}]}`, space, link)
				} else {
					w.Header().Set("Content-Type", "text/html")
					_, _ = fmt.Fprintf(w, `<a href="%s">pkg</a>`, link)
				}
			}))
			defer upstream.Close()
			for _, prefix := range []string{"", "/pypi"} {
				info := testutils.SiteInfo("pypi", config.SiteModePypiProxy, prefix, "")
				h, err := NewProxyHandler(info, testutils.NewRequestHelper(t), &config.PypiRegistrySettings{
					UpstreamSimpleUrl: configStringPtr(upstream.URL + tc.simplePath),
					UpstreamFilesUrl:  configStringPtr(upstream.URL + tc.filesPath),
				})
				require.NoError(t, err)
				w := httptest.NewRecorder()
				h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, prefix+"/simple/demo/", nil))
				require.Equal(t, http.StatusOK, w.Code)
				expected := prefix + "/files" + tc.fileSuffix + "?download=1#sha256=abc"
				if tc.link == "pkg.whl" {
					expected = tc.link + "?download=1#sha256=abc"
				}
				if tc.json {
					var body struct {
						Files []struct{ URL string } `json:"files"`
					}
					require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
					require.Len(t, body.Files, 1)
					require.Equal(t, expected, body.Files[0].URL)
				} else {
					require.Equal(t, `<a href="`+expected+`">pkg</a>`, w.Body.String())
				}
				baseURL, err := url.Parse("http://pavonis.local" + prefix + "/simple/demo/")
				require.NoError(t, err)
				fileURL, err := baseURL.Parse(expected)
				require.NoError(t, err)
				fileURL.Fragment = ""
				w = httptest.NewRecorder()
				h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, fileURL.String(), nil))
				require.Equal(t, http.StatusOK, w.Code)
				require.Equal(t, "wheel contents", w.Body.String())
			}
		})
	}
}

func TestServeHttpLeavesNonSimpleContentTypeUnchanged(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"http://upstream/files/demo.whl"}`))
	}))
	defer upstream.Close()

	h := newPypiTestHandler(t, upstream.URL)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/pypi/simple/demo", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `http://upstream/files/demo.whl`)
}

func TestServeHttpLeavesUnmatchedLinksUnchanged(t *testing.T) {
	body := `<a href="https://other.example/files/a.whl">a</a><a href="/files-other/b.whl">b</a><a href="//other.example/c.whl">c</a>`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
	}))
	defer upstream.Close()
	for _, filesPath := range []string{"/files", ""} {
		h, err := NewProxyHandler(testutils.SiteInfo("pypi", config.SiteModePypiProxy, "/pypi", ""), testutils.NewRequestHelper(t), &config.PypiRegistrySettings{
			UpstreamSimpleUrl: configStringPtr(upstream.URL + "/simple"),
			UpstreamFilesUrl:  configStringPtr(upstream.URL + filesPath),
		})
		require.NoError(t, err)
		w := httptest.NewRecorder()
		h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/pypi/simple/demo/", nil))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, body, w.Body.String())
	}
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

func newPypiTestHandler(t *testing.T, upstream string) *proxyHandler {
	t.Helper()
	info := testutils.SiteInfo("pypi", config.SiteModePypiProxy, "/pypi", "")
	h, err := NewProxyHandler(info, testutils.NewRequestHelper(t), &config.PypiRegistrySettings{
		UpstreamSimpleUrl: configStringPtr(upstream + "/simple"),
		UpstreamFilesUrl:  configStringPtr(upstream + "/files"),
	})
	require.NoError(t, err)
	return h.(*proxyHandler)
}

func configStringPtr(value string) *string { return &value }
