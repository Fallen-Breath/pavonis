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
	_, err := NewProxyHandler(&handler.Info{Id: "http", PathPrefix: "/"}, nil, &config.HttpGeneralProxySettings{
		Mappings:       []*config.HttpGeneralProxyMapping{{Path: "/", Destination: "not-a-url"}},
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
		Mappings: []*config.HttpGeneralProxyMapping{
			{Path: "/", Destination: upstream.URL + "/base"},
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
