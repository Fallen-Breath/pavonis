package crproxy

import (
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	servercontext "github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }
func testHelper(t *testing.T) (*common.RequestHelper, func()) {
	cfg := &config.Config{}
	require.NoError(t, cfg.Init())
	f, e := common.NewRequestHelperFactory(cfg)
	require.NoError(t, e)
	return f.NewRequestHelper(nil), f.Shutdown
}
func TestSingleProxyServeHttpForwardsV2(t *testing.T) {
	var gotPath string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
		_, _ = w.Write([]byte("manifest"))
	}))
	defer up.Close()
	helper, done := testHelper(t)
	defer done()
	mode := config.SiteModeContainerRegistrySingleProxy
	site := &config.SiteConfig{Id: "cr", Mode: &mode, Host: config.SiteHosts{"*"}, PathPrefix: "/r", SelfUrl: "http://proxy.test"}
	info := handler.NewSiteInfo(site.Id, site)
	h, e := NewContainerRegistrySingleProxyHandler(info, helper, &config.ContainerRegistrySingleProxySettings{UpstreamV2Url: strPtr(up.URL), Auth: &config.ContainerRegistryAuthConfig{}, AllowPush: boolPtr(true), AllowList: boolPtr(true)})
	require.NoError(t, e)
	defer h.Shutdown()
	r := httptest.NewRequest(http.MethodGet, "/r/v2/library/alpine/manifests/latest", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(servercontext.NewRequestContext("x", "127.0.0.1"), w, r)
	require.Equal(t, 200, w.Code)
	require.Equal(t, "/library/alpine/manifests/latest", gotPath)
	require.Equal(t, "manifest", w.Body.String())
}
func TestSingleProxyRejectsPushAndNonWhitelisted(t *testing.T) {
	helper, done := testHelper(t)
	defer done()
	mode := config.SiteModeContainerRegistrySingleProxy
	site := &config.SiteConfig{Id: "cr", Mode: &mode, Host: config.SiteHosts{"*"}, PathPrefix: "/r", SelfUrl: "http://proxy.test"}
	info := handler.NewSiteInfo(site.Id, site)
	h, e := NewContainerRegistrySingleProxyHandler(info, helper, &config.ContainerRegistrySingleProxySettings{UpstreamV2Url: strPtr("http://127.0.0.1:1"), Auth: &config.ContainerRegistryAuthConfig{}, AllowPush: boolPtr(false), AllowList: boolPtr(true), ReposWhitelist: []string{"library/alpine"}})
	require.NoError(t, e)
	defer h.Shutdown()
	r := httptest.NewRequest(http.MethodPut, "/r/v2/library/alpine/manifests/latest", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(servercontext.NewRequestContext("x", "127.0.0.1"), w, r)
	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
	r = httptest.NewRequest(http.MethodGet, "/r/v2/library/ubuntu/manifests/latest", nil)
	w = httptest.NewRecorder()
	h.ServeHttp(servercontext.NewRequestContext("x", "127.0.0.1"), w, r)
	require.Equal(t, http.StatusForbidden, w.Code)
}
func TestAnyRegistryProxyRoutesHost(t *testing.T) {
	var gotPath string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer up.Close()
	helper, done := testHelper(t)
	defer done()
	mode := config.SiteModeContainerRegistryAnyProxy
	site := &config.SiteConfig{Id: "cra", Mode: &mode, Host: config.SiteHosts{"*"}, PathPrefix: "/r", SelfUrl: "http://proxy.test"}
	info := handler.NewSiteInfo(site.Id, site)
	h, e := NewContainerRegistryAnyProxyHandler(info, helper, &config.ContainerRegistryAnyProxySettings{Auth: &config.ContainerRegistryAuthConfig{}, AllowList: boolPtr(true)})
	require.NoError(t, e)
	defer h.Shutdown()
	// Any proxy routes to https registry host; use invalid host to validate routing before network.
	r := httptest.NewRequest(http.MethodGet, "/r/bad_host/v2/library/alpine/manifests/latest", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(servercontext.NewRequestContext("x", "127.0.0.1"), w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	_ = up
	_ = gotPath
}
