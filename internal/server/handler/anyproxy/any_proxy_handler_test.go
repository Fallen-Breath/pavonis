package anyproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestServeHttpForwardsTarget(t *testing.T) {
	var gotPath string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	info := testutils.SiteInfo("any", config.SiteModeAnyProxy, "/proxy", "http://proxy.test")
	h, err := NewHandler(info, testutils.NewRequestHelper(t), &config.AnyProxySettings{})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/proxy"+upstream.URL+"/hello?x=1", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "/hello", gotPath)
	require.Equal(t, "ok", w.Body.String())
}

func TestServeHttpRejectsAuthAndBlacklist(t *testing.T) {
	info := testutils.SiteInfo("any", config.SiteModeAnyProxy, "/proxy", "http://proxy.test")
	h, err := NewHandler(info, testutils.NewRequestHelper(t), &config.AnyProxySettings{
		DomainBlacklist: []string{"blocked.test"},
		Auth: &config.ContainerRegistryAuthConfig{
			Enabled: true,
			Users:   []*config.User{{Name: "u", Password: "p"}},
		},
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/proxy/https://example.test/x", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	r = httptest.NewRequest(http.MethodGet, "/proxy/https://blocked.test/x", nil)
	r.SetBasicAuth("u", "p")
	w = httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)
	require.Equal(t, http.StatusForbidden, w.Code)
}
