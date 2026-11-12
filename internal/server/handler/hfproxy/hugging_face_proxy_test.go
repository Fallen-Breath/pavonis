package hfproxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestServeHttpForwardsAndRewritesHfRedirect(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", upstreamURLForTest+"/org/model/resolve/abc/file")
		w.WriteHeader(http.StatusFound)
	}))
	defer upstream.Close()
	oldURL := hfUrl
	hfUrl, _ = url.Parse(upstream.URL)
	defer func() { hfUrl = oldURL }()
	upstreamURLForTest = upstream.URL

	info := testutils.SiteInfo("hf", config.SiteModeHuggingFaceProxy, "/hf", "https://proxy.example.test")
	h, err := NewHuggingFaceProxyHandler(info, testutils.NewRequestHelper(t), &config.HuggingFaceProxySettings{})
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodGet, "/hf/org/model/resolve/abc/file", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)
	require.Equal(t, http.StatusFound, w.Code)
	require.Equal(t, "https://proxy.example.test/hf/org/model/resolve/abc/file", w.Header().Get("Location"))
}

var upstreamURLForTest string

func TestIsValidHfPath(t *testing.T) {
	for _, path := range []string{
		"/api/models/org/model/revision/main",
		"/api/datasets/org/data/revision/main",
		"/org/model/resolve/abcdef/file.bin",
		"/datasets/org/data/resolve/012345/data.json",
		"/org/model/resolve/main/file.bin",
	} {
		require.True(t, isValidHfPath(path), path)
	}
	for _, path := range []string{"", "/", "/org/model/blob/main/file"} {
		require.False(t, isValidHfPath(path), path)
	}
}

func TestTryRewriteUrlToSelf(t *testing.T) {
	self := mustUrl("https://proxy.example.test")
	for _, pathPrefix := range []string{"", "/hf"} {
		t.Run(fmt.Sprintf("pathPrefix=%q", pathPrefix), func(t *testing.T) {
			h := &proxyHandler{selfUrl: self, info: &handler.Info{PathPrefix: pathPrefix}}
			for _, path := range []string{"/org/model/resolve/abc/file", "/org/model/resolve/refs%2Fheads%2Fmain/file"} {
				for _, host := range []string{"huggingface.co", "us.aws.cdn.hf.co"} {
					t.Run(host+path, func(t *testing.T) {
						upstream := mustUrl("https://" + host + path + "?download=true#section")
						original := upstream.String()
						got := h.tryRewriteUrlToSelf(upstream)
						prefix := pathPrefix
						if host != "huggingface.co" {
							prefix += "/.hf-storage/" + host
						}
						require.Equal(t, "https://proxy.example.test"+prefix+path+"?download=true#section", got.String())
						require.Equal(t, original, upstream.String())
					})
				}
			}
			require.Nil(t, h.tryRewriteUrlToSelf(mustUrl("https://example.test/file")))
		})
	}
}

func TestServeHttpPreservesEscapedPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.RequestURI)
	}))
	defer upstream.Close()
	upstreamUrl := mustUrl(upstream.URL)
	for _, pathPrefix := range []string{"", "/hf"} {
		t.Run(fmt.Sprintf("pathPrefix=%q", pathPrefix), func(t *testing.T) {
			for _, revision := range []string{"main", "refs%2Fheads%2Fmain"} {
				t.Run(revision, func(t *testing.T) {
					for _, storage := range []bool{false, true} {
						t.Run(fmt.Sprintf("storage=%t", storage), func(t *testing.T) {
							path := "/org/model/resolve/" + revision + "/config.json?download=true&x=a%2Fb"
							requestPath := path
							upstreamHost := "huggingface.co"
							if storage {
								upstreamHost = "us.aws.cdn.hf.co"
								requestPath = "/.hf-storage/" + upstreamHost + path
							}
							var gotHost string
							transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
								gotHost = req.URL.Host
								local := req.Clone(req.Context())
								local.URL.Scheme, local.URL.Host = upstreamUrl.Scheme, upstreamUrl.Host
								return http.DefaultTransport.RoundTrip(local)
							})
							info := testutils.SiteInfo("hf", config.SiteModeHuggingFaceProxy, pathPrefix, "https://proxy.example.test")
							h, err := NewHuggingFaceProxyHandler(info, testutils.NewRequestHelperWithTransport(t, transport), &config.HuggingFaceProxySettings{})
							require.NoError(t, err)
							prefix := (&url.URL{Path: pathPrefix}).EscapedPath()
							r := httptest.NewRequest(http.MethodGet, prefix+requestPath, nil)
							w := httptest.NewRecorder()
							h.ServeHttp(testutils.Context(), w, r)
							require.Equal(t, http.StatusOK, w.Code)
							require.Equal(t, path, w.Body.String())
							require.Equal(t, upstreamHost, gotHost)
						})
					}
				})
			}
		})
	}
}

func TestRewriteHfResponseHeadersPreservesEscapedPath(t *testing.T) {
	for _, pathPrefix := range []string{"", "/hf"} {
		t.Run(fmt.Sprintf("pathPrefix=%q", pathPrefix), func(t *testing.T) {
			h := &proxyHandler{selfUrl: mustUrl("https://proxy.example.test"), info: &handler.Info{PathPrefix: pathPrefix}}
			for _, path := range []string{"/org/model/resolve/main/config.json", "/org/model/resolve/refs%2Fheads%2Fmain/config.json"} {
				t.Run(path, func(t *testing.T) {
					path += "?download=true"
					resp := &http.Response{StatusCode: http.StatusFound, Header: make(http.Header)}
					resp.Header.Set("Location", path)
					resp.Header.Set("Link", "<https://huggingface.co"+path+">; rel=\"next\"")
					h.rewriteHfResponseHeaders(testutils.Context(), resp)
					require.Equal(t, pathPrefix+path, resp.Header.Get("Location"))
					require.Equal(t, "<https://proxy.example.test"+pathPrefix+path+">; rel=\"next\"", resp.Header.Get("Link"))
				})
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mustUrl(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
