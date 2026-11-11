package httpproxy

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

func TestServeHttpForwardsMappedRequest(t *testing.T) {
	var gotPath, gotHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotHeader = r.Header.Get("X-Test")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	helper := testutils.NewRequestHelper(t)
	info := testutils.SiteInfo("http", config.SiteModeHttpGeneralProxy, "/proxy", "")
	destination, err := url.Parse(upstream.URL)
	require.NoError(t, err)
	action := config.RedirectActionNone
	h, err := NewProxyHandler(info, helper, &config.HttpGeneralProxySettings{
		Mappings:       []*config.HttpGeneralProxyMapping{{Path: "/api", Destination: destination.String()}},
		RedirectAction: &action,
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/proxy/api/hello", nil)
	r.Header.Set("X-Test", "present")
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "/hello", gotPath)
	require.Equal(t, "present", gotHeader)
	require.Equal(t, "upstream response", w.Body.String())
}

func TestNewProxyHandlerRejectsInvalidDestination(t *testing.T) {
	action := config.RedirectActionNone
	_, err := NewProxyHandler(&handler.Info{Id: "http"}, nil, &config.HttpGeneralProxySettings{
		Mappings:       []*config.HttpGeneralProxyMapping{{Path: "/a", Destination: "not-a-url"}},
		RedirectAction: &action,
	})
	require.Error(t, err)
	require.Contains(t, fmt.Sprint(err), "invalid destination URL")
}

func TestServeHttpUsesLongestMapping(t *testing.T) {
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/special/item", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	action := config.RedirectActionNone
	h, err := NewProxyHandler(testutils.SiteInfo("http", config.SiteModeHttpGeneralProxy, "/proxy", "http://proxy.test"), testutils.NewRequestHelper(t), &config.HttpGeneralProxySettings{
		Destination: upstream.URL + "/base",
		Mappings: []*config.HttpGeneralProxyMapping{
			{Path: "/special", Destination: upstream.URL + "/special"},
		},
		RedirectAction: &action,
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/proxy/special/item", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestServeHttpRejectsUnmappedPath(t *testing.T) {
	action := config.RedirectActionNone
	h, err := NewProxyHandler(testutils.SiteInfo("http", config.SiteModeHttpGeneralProxy, "/proxy", "http://proxy.test"), testutils.NewRequestHelper(t), &config.HttpGeneralProxySettings{
		Mappings:       []*config.HttpGeneralProxyMapping{{Path: "/special", Destination: "http://upstream.test"}},
		RedirectAction: &action,
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/proxy/specialx", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestRewriteLocation(t *testing.T) {
	action := config.RedirectActionRewriteOnly
	hdl, err := NewProxyHandler(testutils.SiteInfo("http", config.SiteModeHttpGeneralProxy, "/proxy", "http://proxy.test"), testutils.NewRequestHelper(t), &config.HttpGeneralProxySettings{
		Mappings: []*config.HttpGeneralProxyMapping{
			{Path: "/a", Destination: "https://upstream.test/api"},
			{Path: "/b", Destination: "https://upstream.test/api/"},
		},
		RedirectAction: &action,
	})
	require.NoError(t, err)
	h := hdl.(*proxyHandler)
	mappingOf := func(path string) *mapping {
		for _, m := range h.mappings {
			if m.PathPrefix == path {
				return m
			}
		}
		t.Fatalf("mapping %s not found", path)
		return nil
	}

	for _, tc := range []struct {
		name     string
		mapping  string
		location string
		want     string
	}{
		{"same origin", "/a", "https://upstream.test/api/foo?x=1", "/proxy/a/foo?x=1"},
		{"relative", "/a", "/api/foo", "/proxy/a/foo"},
		{"destination root", "/a", "https://upstream.test/api", "/proxy/a"},
		{"destination with trailing slash", "/b", "https://upstream.test/api/foo", "/proxy/b/foo"},
		{"sibling path", "/a", "https://upstream.test/apix/foo", ""},
		{"outside destination", "/a", "https://upstream.test/other", ""},
		{"other host", "/a", "https://other.test/api/foo", ""},
		{"other scheme", "/a", "http://upstream.test/api/foo", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {tc.location}}}
			got := h.rewriteLocation(testutils.Context(), mappingOf(tc.mapping), resp)
			if tc.want == "" {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.Equal(t, tc.want, *got)
			}
		})
	}
}
