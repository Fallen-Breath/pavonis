package pypiproxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
