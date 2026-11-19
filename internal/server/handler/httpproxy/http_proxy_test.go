package httpproxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestServeHttpForwardsMappedRequest(t *testing.T) {
	var gotRequestURI, gotHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		gotHeader = r.Header.Get("X-Test")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	for _, pathPrefix := range []string{"", "/proxy"} {
		t.Run(fmt.Sprintf("pathPrefix=%q", pathPrefix), func(t *testing.T) {
			for _, path := range []string{"/hello", "/projects/team%2Fapp"} {
				t.Run(path, func(t *testing.T) {
					info := testutils.SiteInfo("http", config.SiteModeHttp, pathPrefix, "")
					action := config.RedirectActionNone
					h, err := NewHttpHandler(info, testutils.NewRequestHelper(t), &config.HttpSiteSettings{
						Mappings:       []*config.HttpSiteMapping{{Path: "/api", Destination: upstream.URL + "/api/v4"}},
						RedirectAction: &action,
					})
					require.NoError(t, err)

					r := httptest.NewRequest(http.MethodGet, pathPrefix+"/api"+path+"?ref=main", nil)
					r.Header.Set("X-Test", "present")
					w := httptest.NewRecorder()
					h.ServeHttp(testutils.Context(), w, r)

					require.Equal(t, http.StatusAccepted, w.Code)
					require.Equal(t, "/api/v4"+path+"?ref=main", gotRequestURI)
					require.Equal(t, "present", gotHeader)
					require.Equal(t, "upstream response", w.Body.String())
				})
			}
		})
	}
}

func TestNewHttpHandlerRejectsInvalidDestination(t *testing.T) {
	action := config.RedirectActionNone
	_, err := NewHttpHandler(&handler.Info{Id: "http"}, nil, &config.HttpSiteSettings{
		Mappings:       []*config.HttpSiteMapping{{Path: "/a", Destination: "not-a-url"}},
		RedirectAction: &action,
	})
	require.Error(t, err)
}

func TestServeHttpUsesLongestMapping(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, r.RequestURI)
	}))
	defer upstream.Close()

	action := config.RedirectActionNone
	h, err := NewHttpHandler(testutils.SiteInfo("http", config.SiteModeHttp, "/proxy", "http://proxy.test"), testutils.NewRequestHelper(t), &config.HttpSiteSettings{
		Destination: upstream.URL + "/base",
		Mappings: []*config.HttpSiteMapping{
			{Path: "/p", Destination: upstream.URL + "/api"},
		},
		RedirectAction: &action,
	})
	require.NoError(t, err)

	for _, tc := range []struct{ path, wantURI string }{
		{"/proxy/p/team%2fops", "/api/team%2fops"},
		{"/proxy/%70/team%2fops", "/base/%70/team%2fops"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, tc.wantURI, w.Body.String())
		})
	}
}

func TestServeHttpRejectsUnmappedPath(t *testing.T) {
	action := config.RedirectActionNone
	h, err := NewHttpHandler(testutils.SiteInfo("http", config.SiteModeHttp, "/proxy", "http://proxy.test"), testutils.NewRequestHelper(t), &config.HttpSiteSettings{
		Mappings:       []*config.HttpSiteMapping{{Path: "/special", Destination: "http://upstream.test"}},
		RedirectAction: &action,
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, "/proxy/specialx", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestRewriteOnlyLocation(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", r.Header.Get("X-Test-Location"))
		w.WriteHeader(http.StatusFound)
	}))
	defer upstream.Close()

	action := config.RedirectActionRewriteOnly
	h, err := NewHttpHandler(testutils.SiteInfo("http", config.SiteModeHttp, "/proxy", "http://proxy.test"), testutils.NewRequestHelper(t), &config.HttpSiteSettings{
		Mappings: []*config.HttpSiteMapping{
			{Path: "/a", Destination: upstream.URL + "/api"},
		},
		RedirectAction: &action,
	})
	require.NoError(t, err)

	otherScheme := "https://" + upstream.Listener.Addr().String() + "/api/foo"
	for _, tc := range []struct {
		name     string
		location string
		want     string
	}{
		{"same origin", upstream.URL + "/api/foo?x=1", "/proxy/a/foo?x=1"},
		{"relative", "/api/foo", "/proxy/a/foo"},
		{"destination root", upstream.URL + "/api", "/proxy/a"},
		{"encoded project path", upstream.URL + "/api/projects/team%2Fapp?ref=main", "/proxy/a/projects/team%2Fapp?ref=main"},
		{"relative encoded project path", "/api/projects/team%2Fapp", "/proxy/a/projects/team%2Fapp"},
		{"different prefix spelling", "/%61pi/projects/team%2Fapp", "/%61pi/projects/team%2Fapp"},
		{"sibling path", upstream.URL + "/apix/foo", upstream.URL + "/apix/foo"},
		{"outside destination", upstream.URL + "/other", upstream.URL + "/other"},
		{"other host", "http://other.test/api/foo", "http://other.test/api/foo"},
		{"other scheme", otherScheme, otherScheme},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/proxy/a/start", nil)
			r.Header.Set("X-Test-Location", tc.location)
			w := httptest.NewRecorder()
			h.ServeHttp(testutils.Context(), w, r)
			require.Equal(t, http.StatusFound, w.Code)
			require.Equal(t, tc.want, w.Header().Get("Location"))
		})
	}
}

func TestServeHttpBodyRewriteWiring(t *testing.T) {
	var gotRange string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello upstream"))
	}))
	defer upstream.Close()

	action := config.RedirectActionNone
	h, err := NewHttpHandler(testutils.SiteInfo("http", config.SiteModeHttp, "/proxy", ""), testutils.NewRequestHelper(t), &config.HttpSiteSettings{
		Mappings:       []*config.HttpSiteMapping{{Path: "/api", Destination: upstream.URL}},
		RedirectAction: &action,
		BodyRewrites: []*config.HttpSiteBodyRewrite{{
			Paths:        []string{"/api/page"},
			ContentTypes: []string{"text/plain"},
			Replacements: []config.HttpSiteBodyReplacement{{Search: "upstream", Replace: "proxy"}},
		}},
	})
	require.NoError(t, err)

	for _, tc := range []struct {
		name      string
		path      string
		wantRange string
		wantBody  string
	}{
		{"matched path", "/proxy/api/page", "", "hello proxy"},
		{"different path spelling", "/proxy/api/%70age", "bytes=0-3", "hello upstream"},
		{"unmatched path", "/proxy/api/other", "bytes=0-3", "hello upstream"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tc.path, nil)
			r.Header.Set("Range", "bytes=0-3")
			w := httptest.NewRecorder()
			h.ServeHttp(testutils.Context(), w, r)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, tc.wantRange, gotRange)
			require.Equal(t, tc.wantBody, w.Body.String())
		})
	}
}
