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
