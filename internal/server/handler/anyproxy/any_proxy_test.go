package anyproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestIsAllowedMethod(t *testing.T) {
	h := &proxyHandler{settings: &config.AnyProxySettings{}}
	require.True(t, h.isAllowedMethod(http.MethodGet))
	require.False(t, h.isAllowedMethod(http.MethodPost))
	h.settings.AllowedMethods = []string{"get", "POST"}
	require.True(t, h.isAllowedMethod(http.MethodGet))
	require.True(t, h.isAllowedMethod(http.MethodPost))
	require.False(t, h.isAllowedMethod(http.MethodDelete))
}

func TestIsBlacklisted(t *testing.T) {
	h := &proxyHandler{settings: &config.AnyProxySettings{DomainBlacklist: []string{
		"example.com", "*.blocked.test", "  ignored.example. ",
	}}}
	require.True(t, h.isBlacklisted("EXAMPLE.COM."))
	require.True(t, h.isBlacklisted("a.blocked.test"))
	require.False(t, h.isBlacklisted("blocked.test"))
	require.True(t, h.isBlacklisted("ignored.example"))
	require.False(t, h.isBlacklisted("other.test"))
}

func TestCheckAuthenticateRejectsMissingAndWrongCredentials(t *testing.T) {
	h := &proxyHandler{settings: &config.AnyProxySettings{Auth: &config.ContainerRegistryAuthConfig{
		Enabled: true,
		Users:   []*config.User{{Name: "u", Password: "p"}},
	}}}

	for _, setup := range []func(*http.Request){nil, func(r *http.Request) { r.SetBasicAuth("u", "wrong") }} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if setup != nil {
			setup(r)
		}
		w := httptest.NewRecorder()

		require.False(t, h.checkAuthenticate(w, r))
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
}

func TestServeHttpRejectsDisallowedMethodWithAllowHeader(t *testing.T) {
	h, err := NewHandler(testutils.SiteInfo("any", config.SiteModeAnyProxy, "/proxy", "http://proxy.test"), testutils.NewRequestHelper(t), &config.AnyProxySettings{
		AllowedMethods: []string{http.MethodGet, http.MethodPost},
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodDelete, "/proxy/https://example.test/x", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)

	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
	require.Equal(t, "GET, POST", w.Header().Get("Allow"))
}
