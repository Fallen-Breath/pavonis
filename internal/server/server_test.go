package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPavonisServerRoutesLongestPathPrefixAndWildcard(t *testing.T) {
	speed := config.SiteModeSpeedTest
	cfg := &config.Config{Sites: []*config.SiteConfig{
		{Id: "short", Host: config.SiteHosts{"proxy.test"}, PathPrefix: "/speed", Mode: &speed},
		{Id: "long", Host: config.SiteHosts{"proxy.test"}, PathPrefix: "/speed/private", Mode: &speed},
		{Id: "fallback", Host: config.SiteHosts{"*"}, PathPrefix: "/", Mode: &speed},
	}}
	require.NoError(t, cfg.Init())
	s, err := NewPavonisServer(cfg)
	require.NoError(t, err)
	defer s.Shutdown()

	tests := []struct {
		name, host, path, want string
	}{
		{"longest prefix", "proxy.test", "/speed/private?bytes=1", "1"},
		{"short prefix", "proxy.test", "/speed?bytes=2", "2"},
		{"wildcard", "other.test", "/anything?bytes=3", "3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tc.path, nil)
			r.Host = tc.host
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, tc.want, w.Header().Get("Content-Length"))
		})
	}
}

func TestPavonisServerReturnsNotFoundWhenNoHandlerMatches(t *testing.T) {
	speed := config.SiteModeSpeedTest
	cfg := &config.Config{Sites: []*config.SiteConfig{{Host: config.SiteHosts{"proxy.test"}, PathPrefix: "/speed", Mode: &speed}}}
	require.NoError(t, cfg.Init())
	s, err := NewPavonisServer(cfg)
	require.NoError(t, err)
	defer s.Shutdown()

	r := httptest.NewRequest(http.MethodGet, "/other", nil)
	r.Host = "proxy.test"
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	require.Equal(t, http.StatusNotFound, w.Code)
}
