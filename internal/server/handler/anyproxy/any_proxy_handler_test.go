package anyproxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

type recordingUpstream struct {
	*httptest.Server
	requests   int
	lastUrl    *url.URL
	lastHeader http.Header
}

func startRecordingUpstream(t *testing.T, handler http.HandlerFunc) *recordingUpstream {
	u := &recordingUpstream{}
	u.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.requests++
		u.lastUrl = r.URL
		u.lastHeader = r.Header.Clone()
		if handler != nil {
			handler(w, r)
		} else {
			_, _ = w.Write([]byte("ok"))
		}
	}))
	t.Cleanup(u.Close)
	return u
}

// hostPort returns the upstream address with the host replaced, e.g. "localhost:12345"
func (u *recordingUpstream) hostPort(t *testing.T, host string) string {
	parsed, err := url.Parse(u.URL)
	require.NoError(t, err)
	return host + ":" + parsed.Port()
}

func newTestHandler(t *testing.T, settings *config.AnyProxySettings) handler.HttpHandler {
	if settings.AllowedMethods == nil {
		settings.AllowedMethods = []string{http.MethodGet}
	}
	return newTestProxyHandler(t, settings)
}

func serve(h handler.HttpHandler, method, target string, setup func(r *http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/proxy/"+target, nil)
	if setup != nil {
		setup(r)
	}
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)
	return w
}

func get(h handler.HttpHandler, target string, setup func(r *http.Request)) *httptest.ResponseRecorder {
	return serve(h, http.MethodGet, target, setup)
}

func withAuth(settings *config.AnyProxySettings) *config.AnyProxySettings {
	settings.Auth = &config.ClientAuthConfig{Enabled: true, Users: []*config.User{{Name: "u", Password: "p"}}}
	return settings
}

func TestServeHttpForwardsTarget(t *testing.T) {
	upstream := startRecordingUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	})
	h := newTestHandler(t, &config.AnyProxySettings{AllowPrivateNetwork: true})

	w := get(h, upstream.URL+"/hello?x=1", nil)
	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "ok", w.Body.String())
	require.Equal(t, "/hello?x=1", upstream.lastUrl.String())
}

func TestServeHttpPreservesEncodedFilenames(t *testing.T) {
	upstream := startRecordingUpstream(t, nil)
	h := newTestHandler(t, &config.AnyProxySettings{AllowPrivateNetwork: true})
	for _, filename := range []string{"report%25.txt", "notes%23final.txt", "question%3F.txt"} {
		t.Run(filename, func(t *testing.T) {
			path := "/" + filename
			w := get(h, upstream.URL+path, nil)
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, path, upstream.lastUrl.RequestURI())
		})
	}
}

func TestServeHttpRejectsForbiddenTarget(t *testing.T) {
	upstream := startRecordingUpstream(t, nil)
	h := newTestHandler(t, &config.AnyProxySettings{AllowPrivateNetwork: true, DomainBlacklist: []string{"127.0.0.1"}})

	require.Equal(t, http.StatusForbidden, get(h, upstream.URL+"/x", nil).Code)
	require.Zero(t, upstream.requests)
}

func TestServeHttpRejectsDisallowedMethodWithAllowHeader(t *testing.T) {
	upstream := startRecordingUpstream(t, nil)
	h := newTestHandler(t, &config.AnyProxySettings{AllowPrivateNetwork: true, AllowedMethods: []string{http.MethodGet, http.MethodPost}})

	w := serve(h, http.MethodDelete, upstream.URL+"/x", nil)
	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
	require.Equal(t, "GET, POST", w.Header().Get("Allow"))
	require.Zero(t, upstream.requests)
}

func TestServeHttpChallengesMissingOrWrongCredentials(t *testing.T) {
	upstream := startRecordingUpstream(t, nil)
	h := newTestHandler(t, withAuth(&config.AnyProxySettings{AllowPrivateNetwork: true}))

	for name, setup := range map[string]func(*http.Request){
		"missing": nil,
		"wrong":   func(r *http.Request) { r.SetBasicAuth("u", "wrong") },
	} {
		t.Run(name, func(t *testing.T) {
			w := get(h, upstream.URL+"/x", setup)
			require.Equal(t, http.StatusUnauthorized, w.Code)
			require.NotEmpty(t, w.Header().Get("WWW-Authenticate"))
		})
	}
	require.Zero(t, upstream.requests)

	w := get(h, upstream.URL+"/x", func(r *http.Request) { r.SetBasicAuth("u", "p") })
	require.Equal(t, http.StatusOK, w.Code)
}

func TestServeHttpUpstreamCredentials(t *testing.T) {
	upstream := startRecordingUpstream(t, nil)
	h := newTestHandler(t, withAuth(&config.AnyProxySettings{AllowPrivateNetwork: true}))

	w := get(h, upstream.URL+"/x", func(r *http.Request) {
		r.SetBasicAuth("u", "p")
		r.Header.Set(headerUpstreamAuthorization, "Bearer token")
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "Bearer token", upstream.lastHeader.Get("Authorization"))
	require.Empty(t, upstream.lastHeader.Values(headerUpstreamAuthorization))

	w = get(h, "http://a:b@"+upstream.hostPort(t, "127.0.0.1")+"/x", func(r *http.Request) {
		r.SetBasicAuth("u", "p")
		r.Header.Set(headerUpstreamAuthorization, "Bearer token")
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "credentials from both the url and the header")
}

func TestServeHttpRenamesUpstreamChallenge(t *testing.T) {
	upstream := startRecordingUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="upstream"`)
		w.WriteHeader(http.StatusUnauthorized)
	})
	h := newTestHandler(t, &config.AnyProxySettings{AllowPrivateNetwork: true})

	w := get(h, upstream.URL+"/x", nil)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Empty(t, w.Header().Values("WWW-Authenticate"))
	require.Equal(t, []string{`Bearer realm="upstream"`}, w.Header().Values(headerUpstreamWwwAuthenticate))
}

func TestServeHttpChecksRedirectTargets(t *testing.T) {
	var redirectTarget string
	upstream := startRecordingUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, redirectTarget, http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("final"))
	})
	h := newTestHandler(t, &config.AnyProxySettings{AllowPrivateNetwork: true, DomainBlacklist: []string{"localhost"}})

	redirectTarget = "/final"
	w := get(h, upstream.URL+"/redirect", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "final", w.Body.String())

	redirectTarget = "http://" + upstream.hostPort(t, "localhost") + "/final"
	w = get(h, upstream.URL+"/redirect", nil)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestServeHttpPrivateNetwork(t *testing.T) {
	upstream := startRecordingUpstream(t, nil)

	require.Equal(t, http.StatusForbidden, get(newTestHandler(t, &config.AnyProxySettings{}), upstream.URL+"/x", nil).Code)
	require.Zero(t, upstream.requests)
	require.Equal(t, http.StatusOK, get(newTestHandler(t, &config.AnyProxySettings{AllowPrivateNetwork: true}), upstream.URL+"/x", nil).Code)
}

func TestServeHttpSizeLimit(t *testing.T) {
	body := strings.Repeat("a", 100_000)
	upstream := startRecordingUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write([]byte(body))
	})
	h := newTestHandler(t, &config.AnyProxySettings{AllowPrivateNetwork: true, SizeLimit: 1000})

	require.Equal(t, http.StatusForbidden, get(h, upstream.URL+"/x", nil).Code)
}
