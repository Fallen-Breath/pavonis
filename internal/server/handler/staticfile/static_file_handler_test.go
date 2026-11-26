package staticfile

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/server/resourcelimit"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/stretchr/testify/require"
)

type innerHandler struct {
	info   *handler.Info
	served bool
}

func (h *innerHandler) Info() *handler.Info { return h.info }
func (h *innerHandler) Shutdown()           {}
func (h *innerHandler) ServeHttp(_ *context.RequestContext, w http.ResponseWriter, _ *http.Request) {
	h.served = true
	w.WriteHeader(http.StatusTeapot)
}

func newTestPolicy(t *testing.T, perClient config.ClientQuotaConfig) *resourcelimit.Policy {
	t.Helper()
	manager, err := resourcelimit.NewManager(&config.ResourceLimitConfig{
		MaxConcurrent429Responses: utils.ToPtr(1),
		IdleClientCacheSize:       utils.ToPtr(1),
		Global:                    config.ResourceQuotaConfig{PerClient: perClient},
	})
	require.NoError(t, err)
	t.Cleanup(manager.Shutdown)
	return manager.CreateSitePolicy("test", "")
}

func newTestHandler(t *testing.T, pathPrefix string, policy *resourcelimit.Policy, staticFiles ...*config.StaticFileConfig) (handler.HttpHandler, *innerHandler) {
	t.Helper()
	if policy == nil {
		policy = newTestPolicy(t, config.ClientQuotaConfig{})
	}
	inner := &innerHandler{info: testutils.NewSiteInfo("test", config.SiteModeSpeedTest, pathPrefix, "")}
	return WrapHandler(inner, policy, staticFiles), inner
}

func serve(h handler.HttpHandler, method, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.NewRequestContext(), w, httptest.NewRequest(method, target, nil))
	return w
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestServesFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "index.html")
	writeFile(t, file, "<h1>hi</h1>")
	h, _ := newTestHandler(t, "", nil, &config.StaticFileConfig{Path: "/", File: file})

	w := serve(h, http.MethodGet, "/")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "<h1>hi</h1>", w.Body.String())
}

func TestServesContentAsPlainTextByDefault(t *testing.T) {
	content := "ok"
	h, _ := newTestHandler(t, "", nil, &config.StaticFileConfig{Path: "/healthz", Content: &content})

	w := serve(h, http.MethodGet, "/healthz")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "ok", w.Body.String())
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestContentTypeOverridesDefault(t *testing.T) {
	file := filepath.Join(t.TempDir(), "index.html")
	writeFile(t, file, "x")
	content := "{}"
	h, _ := newTestHandler(t, "", nil,
		&config.StaticFileConfig{Path: "/file", File: file, ContentType: "text/x-test"},
		&config.StaticFileConfig{Path: "/content", Content: &content, ContentType: "application/json"},
	)

	require.Equal(t, "text/x-test", serve(h, http.MethodGet, "/file").Header().Get("Content-Type"))
	require.Equal(t, "application/json", serve(h, http.MethodGet, "/content").Header().Get("Content-Type"))
}

func TestMatchesExactPathIgnoringQuery(t *testing.T) {
	content := "ok"
	tests := []struct {
		target     string
		wantStatic bool
	}{
		{"/a?v=1", true},
		{"/a/", false},
	}
	for _, tc := range tests {
		t.Run(tc.target, func(t *testing.T) {
			h, inner := newTestHandler(t, "", nil, &config.StaticFileConfig{Path: "/a", Content: &content})
			serve(h, http.MethodGet, tc.target)
			require.Equal(t, !tc.wantStatic, inner.served)
		})
	}
}

func TestSitePrefixItselfMatchesRoot(t *testing.T) {
	content := "ok"
	h, inner := newTestHandler(t, "/x", nil, &config.StaticFileConfig{Path: "/", Content: &content})

	w := serve(h, http.MethodGet, "/x")
	require.False(t, inner.served)
	require.Equal(t, "ok", w.Body.String())
}

func TestAllowsOnlyGetAndHead(t *testing.T) {
	content := "ok"
	h, inner := newTestHandler(t, "", nil, &config.StaticFileConfig{Path: "/a", Content: &content})

	require.Equal(t, http.StatusOK, serve(h, http.MethodHead, "/a").Code)
	require.Equal(t, http.StatusMethodNotAllowed, serve(h, http.MethodPost, "/a").Code)
	require.False(t, inner.served)
}

func TestAppliesRequestRateLimit(t *testing.T) {
	policy := newTestPolicy(t, config.ClientQuotaConfig{RequestPerHour: utils.ToPtr(1.0)})
	content := "ok"
	h, _ := newTestHandler(t, "", policy, &config.StaticFileConfig{Path: "/a", Content: &content})

	require.Equal(t, http.StatusOK, serve(h, http.MethodGet, "/a").Code)
	require.Equal(t, http.StatusTooManyRequests, serve(h, http.MethodGet, "/a").Code)
}

func TestReadsFileOnEachRequest(t *testing.T) {
	file := filepath.Join(t.TempDir(), "index.html")
	writeFile(t, file, "old")
	h, _ := newTestHandler(t, "", nil, &config.StaticFileConfig{Path: "/", File: file})
	require.Equal(t, "old", serve(h, http.MethodGet, "/").Body.String())

	writeFile(t, file, "new")
	require.Equal(t, "new", serve(h, http.MethodGet, "/").Body.String())
}

func TestMissingFileIsServerError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "index.html")
	writeFile(t, file, "x")
	h, inner := newTestHandler(t, "", nil, &config.StaticFileConfig{Path: "/", File: file})
	require.NoError(t, os.Remove(file))

	require.Equal(t, http.StatusInternalServerError, serve(h, http.MethodGet, "/").Code)
	require.False(t, inner.served)
}
