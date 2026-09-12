package crproxy

import (
	"github.com/Fallen-Breath/pavonis/internal/config"
	servercontext "github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }
func TestSingleProxyServeHttpForwardsV2(t *testing.T) {
	var gotPath string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
		_, _ = w.Write([]byte("manifest"))
	}))
	defer up.Close()
	helper := testutils.NewRequestHelper(t)
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
	helper := testutils.NewRequestHelper(t)
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
	helper := testutils.NewRequestHelper(t)
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
}

func TestSingleProxyRewritesAuthRealm(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Www-Authenticate", `Bearer realm="http://`+r.Host+`/token",service="registry"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer up.Close()

	helper := testutils.NewRequestHelper(t)
	mode := config.SiteModeContainerRegistrySingleProxy
	site := &config.SiteConfig{Id: "cr", Mode: &mode, Host: config.SiteHosts{"*"}, PathPrefix: "/r", SelfUrl: "http://proxy.test"}
	info := handler.NewSiteInfo(site.Id, site)
	h, err := NewContainerRegistrySingleProxyHandler(info, helper, &config.ContainerRegistrySingleProxySettings{
		UpstreamV2Url: strPtr(up.URL),
		Auth:          &config.ContainerRegistryAuthConfig{},
		AllowPush:     boolPtr(true),
		AllowList:     boolPtr(true),
	})
	require.NoError(t, err)
	defer h.Shutdown()

	r := httptest.NewRequest(http.MethodGet, "/r/v2/library/alpine/manifests/latest", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(servercontext.NewRequestContext("x", "127.0.0.1"), w, r)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, `Bearer realm="http://proxy.test/r/auth",service="registry"`, w.Header().Get("Www-Authenticate"))
}

func TestSingleProxyRewritesUploadLocation(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://"+r.Host+"/v2/library/alpine/blobs/uploads/abc")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer up.Close()

	helper := testutils.NewRequestHelper(t)
	mode := config.SiteModeContainerRegistrySingleProxy
	site := &config.SiteConfig{Id: "cr", Mode: &mode, Host: config.SiteHosts{"*"}, PathPrefix: "/r", SelfUrl: "http://proxy.test"}
	info := handler.NewSiteInfo(site.Id, site)
	h, err := NewContainerRegistrySingleProxyHandler(info, helper, &config.ContainerRegistrySingleProxySettings{
		UpstreamV2Url: strPtr(up.URL),
		Auth:          &config.ContainerRegistryAuthConfig{},
		AllowPush:     boolPtr(true),
		AllowList:     boolPtr(true),
	})
	require.NoError(t, err)
	defer h.Shutdown()

	r := httptest.NewRequest(http.MethodPost, "/r/v2/library/alpine/blobs/uploads/", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(servercontext.NewRequestContext("x", "127.0.0.1"), w, r)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "http://proxy.test/r/v2/library/alpine/blobs/uploads/abc", w.Header().Get("Location"))
}

func TestSingleProxyV1AllowsSupportedListingEndpoints(t *testing.T) {
	var gotPath string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("search"))
	}))
	defer up.Close()

	helper := testutils.NewRequestHelper(t)
	mode := config.SiteModeContainerRegistrySingleProxy
	site := &config.SiteConfig{Id: "cr-v1", Mode: &mode, Host: config.SiteHosts{"*"}, PathPrefix: "/r", SelfUrl: "http://proxy.test"}
	info := handler.NewSiteInfo(site.Id, site)
	h, err := NewContainerRegistrySingleProxyHandler(info, helper, &config.ContainerRegistrySingleProxySettings{
		UpstreamV1Url: strPtr(up.URL),
		UpstreamV2Url: strPtr(up.URL),
		Auth:          &config.ContainerRegistryAuthConfig{},
		AllowPush:     boolPtr(true),
		AllowList:     boolPtr(true),
	})
	require.NoError(t, err)
	defer h.Shutdown()

	r := httptest.NewRequest(http.MethodGet, "/r/v1/search?q=alpine", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(servercontext.NewRequestContext("x", "127.0.0.1"), w, r)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "/search", gotPath)
	require.Equal(t, "search", w.Body.String())
}

func TestSingleProxyV1RejectsUnsupportedEndpoint(t *testing.T) {
	helper := testutils.NewRequestHelper(t)
	mode := config.SiteModeContainerRegistrySingleProxy
	site := &config.SiteConfig{Id: "cr-v1", Mode: &mode, Host: config.SiteHosts{"*"}, PathPrefix: "/r", SelfUrl: "http://proxy.test"}
	info := handler.NewSiteInfo(site.Id, site)
	h, err := NewContainerRegistrySingleProxyHandler(info, helper, &config.ContainerRegistrySingleProxySettings{
		UpstreamV1Url: strPtr("http://127.0.0.1:1"),
		UpstreamV2Url: strPtr("http://127.0.0.1:1"),
		Auth:          &config.ContainerRegistryAuthConfig{},
		AllowPush:     boolPtr(true),
		AllowList:     boolPtr(true),
	})
	require.NoError(t, err)
	defer h.Shutdown()

	r := httptest.NewRequest(http.MethodGet, "/r/v1/images/search", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(servercontext.NewRequestContext("x", "127.0.0.1"), w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSingleProxyAuthRealmMocksLoginToken(t *testing.T) {
	helper := testutils.NewRequestHelper(t)
	mode := config.SiteModeContainerRegistrySingleProxy
	site := &config.SiteConfig{Id: "cr-auth", Mode: &mode, Host: config.SiteHosts{"*"}, PathPrefix: "/r", SelfUrl: "http://proxy.test"}
	info := handler.NewSiteInfo(site.Id, site)
	auth := &config.ContainerRegistryAuthConfig{Enabled: true, Users: []*config.User{{Name: "user", Password: "pass"}}}
	realm := "http://registry.example/token"
	h, err := NewContainerRegistrySingleProxyHandler(info, helper, &config.ContainerRegistrySingleProxySettings{
		UpstreamAuthRealmUrl: &realm,
		UpstreamV2Url:        strPtr("http://127.0.0.1:1"),
		Auth:                 auth,
		AllowPush:            boolPtr(false),
		AllowList:            boolPtr(true),
	})
	require.NoError(t, err)
	defer h.Shutdown()

	r := httptest.NewRequest(http.MethodGet, "/r/auth", nil)
	r.SetBasicAuth("user", "pass")
	w := httptest.NewRecorder()
	h.ServeHttp(servercontext.NewRequestContext("x", "127.0.0.1"), w, r)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"token": "pavonis-dummy-token"}`, w.Body.String())
}
