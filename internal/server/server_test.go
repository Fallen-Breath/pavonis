package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	servercontext "github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestPavonisServerRoutesLongestPathPrefixAndWildcard(t *testing.T) {
	speed := config.SiteModeSpeedTest
	cfg := &config.Config{Sites: []*config.SiteConfig{
		{Id: "short", Host: config.SiteHosts{"proxy.test"}, PathPrefix: "/speed", Mode: &speed},
		{Id: "long", Host: config.SiteHosts{"proxy.test"}, PathPrefix: "/speed/private", Mode: &speed},
		{Id: "escaped", Host: config.SiteHosts{"proxy.test"}, PathPrefix: "/m%20x", Mode: &speed},
		{Id: "fallback", Host: config.SiteHosts{"*"}, Mode: &speed},
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
		{"escaped site prefix", "proxy.test", "/m%20x/item?bytes=4", "4"},
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

	for _, path := range []string{"/other", "/speedx", "/speed%2Fx"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Host = "proxy.test"
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		require.Equal(t, http.StatusNotFound, w.Code, path)
	}
}

func TestPavonisServerServesStaticFileConfigs(t *testing.T) {
	speed := config.SiteModeSpeedTest
	content := "ok"
	cfg := &config.Config{Sites: []*config.SiteConfig{{
		Host: config.SiteHosts{"proxy.test"}, Mode: &speed,
		StaticFiles: []*config.StaticFileConfig{{Path: "/healthz", Content: &content}},
	}}}
	require.NoError(t, cfg.Init())
	s, err := NewPavonisServer(cfg)
	require.NoError(t, err)
	defer s.Shutdown()

	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.Host = "proxy.test"
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	require.Equal(t, "ok", w.Body.String())
}

func TestSitesShareGroupRateLimitOnStaticFiles(t *testing.T) {
	cfg := &config.Config{ResourceLimit: &config.ResourceLimitConfig{
		Groups: []config.QuotaGroupConfig{{
			Id: "downloads", ResourceQuotaConfig: config.ResourceQuotaConfig{
				AllClients: config.ClientQuotaConfig{RequestPerHour: utils.ToPtr(1.0)},
			},
		}},
	}}
	for _, host := range []string{"a.test", "b.test"} {
		cfg.Sites = append(cfg.Sites, &config.SiteConfig{
			Host: config.SiteHosts{host}, Mode: utils.ToPtr(config.SiteModePypi), QuotaGroup: "downloads",
			StaticFiles: []*config.StaticFileConfig{{Path: "/healthz", Content: utils.ToPtr("ok")}},
		})
	}
	require.NoError(t, cfg.Init())
	s, err := NewPavonisServer(cfg)
	require.NoError(t, err)
	t.Cleanup(s.Shutdown)

	get := func(host string) int {
		r := httptest.NewRequest(http.MethodGet, "http://"+host+"/healthz", nil)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w.Code
	}
	require.Equal(t, http.StatusOK, get("a.test"))
	require.Equal(t, http.StatusTooManyRequests, get("b.test"))
}

func TestCreateHTTPServerUsesConfiguredTimeouts(t *testing.T) {
	cfg := &config.Config{
		Server: &config.ServerConfig{
			ReadHeaderTimeout: utils.ToPtr(3 * time.Second),
			IdleTimeout:       utils.ToPtr(11 * time.Second),
		},
		Sites: []*config.SiteConfig{{Host: config.SiteHosts{"*"}, Mode: utils.ToPtr(config.SiteModeSpeedTest)}},
	}
	require.NoError(t, cfg.Init())
	s := &PavonisServer{cfg: cfg}
	httpServer, err := s.CreateHttpServer()
	require.NoError(t, err)
	require.Equal(t, 3*time.Second, httpServer.server.ReadHeaderTimeout)
	require.Equal(t, 11*time.Second, httpServer.server.IdleTimeout)
}

func TestCreateHTTPServerRejectsInvalidCertificatePair(t *testing.T) {
	dir := t.TempDir()
	a, _ := writeTestCertificate(t, dir, "a.test")
	b, _ := writeTestCertificate(t, dir, "b.test")
	b.KeyFile = a.KeyFile
	cfg := &config.Config{
		Server: &config.ServerConfig{TLS: &config.ServerTLSConfig{Certificates: []config.TLSCertificateConfig{a, b}}},
		Sites:  []*config.SiteConfig{{Host: config.SiteHosts{"*"}, Mode: utils.ToPtr(config.SiteModeSpeedTest)}},
	}
	require.NoError(t, cfg.Init())
	s := &PavonisServer{cfg: cfg}
	_, err := s.CreateHttpServer()
	require.Error(t, err)
}

func TestHTTPServerServesSiteOnlyOverTLS(t *testing.T) {
	dir := t.TempDir()
	a, _ := writeTestCertificate(t, dir, "a.test")
	b, certB := writeTestCertificate(t, dir, "b.test")
	cfg := &config.Config{
		Server: &config.ServerConfig{
			Listen: utils.ToPtr("127.0.0.1:0"),
			TLS:    &config.ServerTLSConfig{Certificates: []config.TLSCertificateConfig{a, b}},
		},
		Sites: []*config.SiteConfig{{
			Host: config.SiteHosts{"b.test"}, Mode: utils.ToPtr(config.SiteModeSpeedTest),
			StaticFiles: []*config.StaticFileConfig{{Path: "/name", Content: utils.ToPtr("site B")}},
		}},
	}
	require.NoError(t, cfg.Init())
	s, err := NewPavonisServer(cfg)
	require.NoError(t, err)
	defer s.Shutdown()
	srv, err := s.CreateHttpServer()
	require.NoError(t, err)
	// Obtain the bound address without reserving and releasing a test port.
	listening := make(chan string, 1)
	srv.server.BaseContext = func(l net.Listener) context.Context {
		listening <- l.Addr().String()
		return context.Background()
	}
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	defer srv.Close()
	var addr string
	select {
	case addr = <-listening:
	case err := <-done:
		t.Fatalf("server stopped before listening: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for listener")
	}
	roots := x509.NewCertPool()
	roots.AddCert(certB)
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: "b.test"}}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport, Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	get := func(scheme string) (*http.Response, error) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, scheme+"://"+addr+"/name", nil)
		require.NoError(t, err)
		req.Host = "b.test"
		return client.Do(req)
	}
	resp, err := get("https")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "site B", string(body))
	resp, err = get("http")
	if err != nil {
		require.True(t, errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET), "unexpected HTTP error: %v", err)
	} else {
		defer resp.Body.Close()
		require.GreaterOrEqual(t, resp.StatusCode, 400)
	}
}

type statusTestHandler struct {
	handler.HttpHandler
	serve func(http.ResponseWriter)
}

var _ handler.HttpHandler = statusTestHandler{}

func (h statusTestHandler) ServeHttp(_ *servercontext.RequestContext, w http.ResponseWriter, _ *http.Request) {
	h.serve(w)
}

func readRequestCountForCode(t *testing.T, site, code string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != "pavonis_server_http_request_total" {
			continue
		}
		for _, m := range family.Metric {
			labels := make(map[string]string)
			for _, label := range m.Label {
				labels[label.GetName()] = label.GetValue()
			}
			if value, ok := labels["code"]; ok && value == code && labels["site"] == site && labels["method"] == http.MethodGet {
				return m.Counter.GetValue()
			}
		}
	}
	return 0
}

func TestServerRequestsCountFinalResponseStatus(t *testing.T) {
	const site = "response-status-test"
	cfg := &config.Config{Sites: []*config.SiteConfig{{
		Id: site, Host: config.SiteHosts{"metrics.test"}, Mode: utils.ToPtr(config.SiteModeSpeedTest),
	}}}
	require.NoError(t, cfg.Init())
	s, err := NewPavonisServer(cfg)
	require.NoError(t, err)
	t.Cleanup(s.Shutdown)
	handlers := s.handlersByHost["metrics.test"]
	original := handlers[0]

	for _, tc := range []struct {
		name     string
		header   int
		abort    bool
		wantCode string
	}{
		{"normal completion without a write", 0, false, "200"},
		{"aborted before writing", 0, true, ""},
		{"aborted after final status", 429, true, "429"},
		{"aborted after early hints only", 103, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handlers[0] = statusTestHandler{original, func(w http.ResponseWriter) {
				if tc.header != 0 {
					w.WriteHeader(tc.header)
				}
				if tc.abort {
					panic(http.ErrAbortHandler)
				}
			}}
			before := readRequestCountForCode(t, site, tc.wantCode)
			serve := func() {
				s.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://metrics.test/", nil))
			}
			if tc.abort {
				require.PanicsWithValue(t, http.ErrAbortHandler, serve)
			} else {
				serve()
			}
			require.Equal(t, before+1, readRequestCountForCode(t, site, tc.wantCode))
		})
	}
}
