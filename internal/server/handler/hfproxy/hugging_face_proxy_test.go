package hfproxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T, pathPrefix string, transport http.RoundTripper) handler.HttpHandler {
	info := testutils.SiteInfo("hf", config.SiteModeHuggingFaceProxy, pathPrefix, "https://proxy.example.test")
	h, err := NewHuggingFaceProxyHandler(info, testutils.NewRequestHelperWithTransport(t, transport), &config.HuggingFaceProxySettings{})
	require.NoError(t, err)
	return h
}

func TestServeHttpRewritesHfRedirect(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		w.Header().Set("Location", "https://huggingface.co/org/model/resolve/abc/file")
		w.WriteHeader(http.StatusFound)
		return w.Result(), nil
	})
	w := httptest.NewRecorder()
	newTestHandler(t, "/hf", transport).ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/hf/org/model/resolve/main/file", nil))
	require.Equal(t, http.StatusFound, w.Code)
	require.Equal(t, "https://proxy.example.test/hf/org/model/resolve/abc/file", w.Header().Get("Location"))
}

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
	const path = "/org/model/resolve/refs%2Fheads%2Fmain/config.json?download=true&x=a%2Fb"
	for _, tc := range []struct {
		requestPath, wantUrl string
	}{
		{"/hf" + path, "https://huggingface.co" + path},
		{"/hf/.hf-storage/us.aws.cdn.hf.co" + path, "https://us.aws.cdn.hf.co" + path},
	} {
		var gotUrl string
		transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotUrl = req.URL.String()
			return httptest.NewRecorder().Result(), nil
		})
		w := httptest.NewRecorder()
		newTestHandler(t, "/hf", transport).ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, tc.requestPath, nil))
		require.Equal(t, http.StatusOK, w.Code, tc.requestPath)
		require.Equal(t, tc.wantUrl, gotUrl, tc.requestPath)
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
