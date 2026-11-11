package hfproxy

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
	h := &proxyHandler{selfUrl: self, info: &handler.Info{PathPrefix: "/hf"}}
	got := h.tryRewriteUrlToSelf(mustUrl("https://huggingface.co/org/model/resolve/abc/file"))
	require.Equal(t, "https://proxy.example.test/hf/org/model/resolve/abc/file", got.String())
	require.Nil(t, h.tryRewriteUrlToSelf(mustUrl("https://example.test/file")))
}

func mustUrl(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
