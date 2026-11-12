package crproxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

const testUpstreamToken = "upstream-token"

// fakeRegistry serves every upstream host. Hosts listed in anonymous accept any request,
// other hosts behave like a token-auth registry whose token service lives at https://auth.test/token
type fakeRegistry struct {
	anonymous      map[string]bool
	withoutService bool

	mu       sync.Mutex
	requests []*http.Request
}

func (f *fakeRegistry) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req.Clone(req.Context()))
	f.mu.Unlock()

	w := httptest.NewRecorder()
	switch {
	case req.URL.Host == "auth.test":
		_, _ = w.Write([]byte(`{"token":"` + testUpstreamToken + `","refresh_token":"r","expires_in":300}`))
	case f.anonymous[req.URL.Host] || req.Header.Get("Authorization") == "Bearer "+testUpstreamToken:
		_, _ = w.Write([]byte("upstream body"))
	case f.withoutService:
		w.Header().Set("Www-Authenticate", `Bearer realm="https://auth.test/token"`)
		w.WriteHeader(http.StatusUnauthorized)
	default:
		w.Header().Set("Www-Authenticate", `Bearer realm="https://auth.test/token",service="svc"`)
		w.WriteHeader(http.StatusUnauthorized)
	}
	resp := w.Result()
	resp.Request = req
	return resp, nil
}

func (f *fakeRegistry) Requests() []*http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*http.Request(nil), f.requests...)
}

func (f *fakeRegistry) LastRequest(t *testing.T) *http.Request {
	requests := f.Requests()
	require.NotEmpty(t, requests)
	return requests[len(requests)-1]
}

func testAuthConfig(enabled bool) *config.ClientAuthConfig {
	if !enabled {
		return &config.ClientAuthConfig{}
	}
	return &config.ClientAuthConfig{Enabled: true, Users: []*config.User{{Name: "alice", Password: "pw"}}}
}

func newTestSingleHandler(t *testing.T, reg *fakeRegistry, authEnabled bool) handler.HttpHandler {
	info := testutils.SiteInfo("cr", config.SiteModeContainerRegistrySingleProxy, "/r", "http://proxy.test")
	h, err := NewContainerRegistrySingleProxyHandler(info, testutils.NewRequestHelperWithTransport(t, reg), &config.ContainerRegistrySingleProxySettings{
		UpstreamV1Url: strPtr("https://registry.test/v1"),
		UpstreamV2Url: strPtr("https://registry.test/v2"),
		Auth:          testAuthConfig(authEnabled),
		AllowPush:     boolPtr(false),
		AllowList:     boolPtr(true),
	})
	require.NoError(t, err)
	t.Cleanup(h.Shutdown)
	return h
}

func newTestAnyHandler(t *testing.T, reg *fakeRegistry, authEnabled bool) handler.HttpHandler {
	info := testutils.SiteInfo("cra", config.SiteModeContainerRegistryAnyProxy, "/r", "http://proxy.test")
	h, err := NewContainerRegistryAnyProxyHandler(info, testutils.NewRequestHelperWithTransport(t, reg), &config.ContainerRegistryAnyProxySettings{
		Auth: testAuthConfig(authEnabled),
	})
	require.NoError(t, err)
	t.Cleanup(h.Shutdown)
	return h
}

func serveRequest(h handler.HttpHandler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)
	return w
}

func newRequest(method, target, authorization string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	if authorization != "" {
		r.Header.Set("Authorization", authorization)
	}
	return r
}

func bearer(token string) string {
	return "Bearer " + token
}

func basicAuth(user, password string) string {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetBasicAuth(user, password)
	return r.Header.Get("Authorization")
}

func requireChallenge(t *testing.T, w *httptest.ResponseRecorder, want map[string]string) {
	t.Helper()
	require.Equal(t, http.StatusUnauthorized, w.Code)
	params, ok := parseBearerChallenge(w.Header().Get("Www-Authenticate"))
	require.True(t, ok)
	require.Equal(t, want, params)
}

func requirePackedTokenResponse(t *testing.T, w *httptest.ResponseRecorder, wantUpstreamToken string) string {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotContains(t, body, "refresh_token")
	tokenString, _ := body["token"].(string)
	token, ok := unpackToken(tokenString)
	require.True(t, ok)
	require.Equal(t, packedToken{Issuer: packedTokenIssuer, User: "alice", Password: "pw", UpstreamToken: wantUpstreamToken}, *token)
	return tokenString
}

func TestSingleProxyAuthRejectsRequestsWithoutPavonisToken(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	for _, authorization := range []string{"", bearer(testUpstreamToken), bearer(packToken("alice", "bad", testUpstreamToken))} {
		w := serveRequest(h, newRequest(http.MethodHead, "/r/v2/library/nginx/manifests/latest", authorization))
		requireChallenge(t, w, map[string]string{"realm": "http://proxy.test/r/auth", "service": "svc", "scope": "repository:library/nginx:pull"})
	}
	for _, req := range reg.Requests() {
		require.Equal(t, "/v2/", req.URL.Path, "the original request must not be forwarded")
		require.Empty(t, req.Header.Get("Authorization"))
	}
}

func TestSingleProxyV1AuthRejectsInvalidCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, authorization string
	}{
		{"anonymous", ""},
		{"wrong Basic password", basicAuth("alice", "bad")},
		{"unknown Basic user", basicAuth("bob", "pw")},
		{"invalid local credentials with upstream credentials", basicAuth("alice$upstream", "bad$secret")},
		{"malformed Basic", "Basic !!!"},
		{"upstream Bearer", bearer(testUpstreamToken)},
		{"invalid Pavonis Bearer", bearer(packToken("alice", "bad", testUpstreamToken))},
		{"legacy Token", "Token upstream-token"},
		{"unsupported scheme", "Digest credentials"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
			h := newTestSingleHandler(t, reg, true)

			for _, path := range []string{"/r/v1/_ping", "/r/v1/search?q=nginx", "/r/v1/repositories/library/nginx/tags"} {
				for _, method := range []string{http.MethodGet, http.MethodHead} {
					r := newRequest(method, path, tc.authorization)
					r.Header.Set("X-Docker-Token", "true")
					w := serveRequest(h, r)
					require.Equal(t, http.StatusUnauthorized, w.Code, method+" "+path)
					require.True(t, strings.HasPrefix(strings.ToLower(w.Header().Get("WWW-Authenticate")), "basic "), "clients must be challenged for Basic credentials")
					require.Empty(t, reg.Requests(), "invalid credentials must not grant access to an anonymous upstream")
				}
			}
		})
	}
}

func TestSingleProxyV1AuthIsolatesBasicCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, authorization, upstreamAuthorization string
	}{
		{"Pavonis only", basicAuth("alice", "pw"), ""},
		{"with upstream credentials", basicAuth("alice$upstream", "pw$secret"), basicAuth("upstream", "secret")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
			h := newTestSingleHandler(t, reg, true)

			for _, path := range []string{"/v1/_ping", "/v1/search?q=nginx", "/v1/repositories/library/nginx/tags"} {
				for _, method := range []string{http.MethodGet, http.MethodHead} {
					w := serveRequest(h, newRequest(method, "/r"+path, tc.authorization))
					require.Equal(t, http.StatusOK, w.Code, method+" "+path)
					req := reg.LastRequest(t)
					require.Equal(t, method, req.Method)
					require.Equal(t, path, req.URL.RequestURI())
					require.Equal(t, tc.upstreamAuthorization, req.Header.Get("Authorization"))
				}
			}
		})
	}
}

func TestSingleProxyV1AuthIsolatesBearerCredentials(t *testing.T) {
	for _, upstreamToken := range []string{"", testUpstreamToken} {
		reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
		h := newTestSingleHandler(t, reg, true)
		authorization := bearer(packToken("alice", "pw", upstreamToken))
		wantAuthorization := ""
		if upstreamToken != "" {
			wantAuthorization = bearer(upstreamToken)
		}

		for _, path := range []string{"/v1/_ping", "/v1/search?q=nginx", "/v1/repositories/library/nginx/tags"} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				w := serveRequest(h, newRequest(method, "/r"+path, authorization))
				require.Equal(t, http.StatusOK, w.Code, method+" "+path)
				req := reg.LastRequest(t)
				require.Equal(t, method, req.Method)
				require.Equal(t, path, req.URL.RequestURI())
				require.Equal(t, wantAuthorization, req.Header.Get("Authorization"))
			}
		}
	}
}

func TestSingleProxyV1ForwardsCredentialsWhenAuthDisabled(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
	h := newTestSingleHandler(t, reg, false)

	for _, authorization := range []string{
		"",
		basicAuth("upstream", "secret"),
		basicAuth("alice$upstream", "pw$secret"),
		bearer(testUpstreamToken),
		bearer(packToken("alice", "pw", testUpstreamToken)),
		"Token upstream-token",
	} {
		for _, path := range []string{"/v1/_ping", "/v1/search?q=nginx", "/v1/repositories/library/nginx/tags"} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				w := serveRequest(h, newRequest(method, "/r"+path, authorization))
				require.Equal(t, http.StatusOK, w.Code, method+" "+path)
				req := reg.LastRequest(t)
				require.Equal(t, method, req.Method)
				require.Equal(t, path, req.URL.RequestURI())
				require.Equal(t, authorization, req.Header.Get("Authorization"))
			}
		}
	}
}

func TestSingleProxyAuthForwardsUpstreamTokenInPavonisToken(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/v2/library/nginx/manifests/latest", bearer(packToken("alice", "pw", testUpstreamToken))))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, bearer(testUpstreamToken), reg.LastRequest(t).Header.Get("Authorization"))

	requestCount := len(reg.Requests())
	w = serveRequest(h, newRequest(http.MethodGet, "/r/v2/", bearer(packToken("alice", "pw", ""))))
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, reg.Requests(), requestCount, "a login-only token is answered by Pavonis")
}

func TestSingleProxyAuthIssuesPavonisToken(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull&service=svc&account=alice", basicAuth("alice", "pw")))
	requirePackedTokenResponse(t, w, testUpstreamToken)
	tokenRequest := reg.LastRequest(t)
	require.Equal(t, "auth.test", tokenRequest.URL.Host)
	require.False(t, tokenRequest.URL.Query().Has("account"))
	require.Empty(t, tokenRequest.Header.Get("Authorization"))

	w = serveRequest(h, newRequest(http.MethodGet, "/r/auth?account=alice&service=svc", basicAuth("alice", "pw")))
	requirePackedTokenResponse(t, w, "")
}

func TestSingleProxyAuthRealmForAnonymousUpstream(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}

	w := serveRequest(newTestSingleHandler(t, reg, true), newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull", basicAuth("alice", "pw")))
	requirePackedTokenResponse(t, w, "")

	w = serveRequest(newTestSingleHandler(t, reg, false), newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull", ""))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSingleProxyAuthRejectsUnsupportedAuthRequests(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	r := newRequest(http.MethodPost, "/r/auth", basicAuth("alice", "pw"))
	r.Body = http.NoBody
	require.Equal(t, http.StatusUnauthorized, serveRequest(h, r).Code)
	require.Equal(t, http.StatusNotFound, serveRequest(h, newRequest(http.MethodGet, "/r/auth/token", basicAuth("alice", "pw"))).Code)
	require.Empty(t, reg.Requests())
}

func TestSingleProxyForwardsAuthPostWithoutAllowPush(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, false)

	r := httptest.NewRequest(http.MethodPost, "/r/auth", strings.NewReader("grant_type=refresh_token&refresh_token=r"))
	w := serveRequest(h, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, http.MethodPost, reg.LastRequest(t).Method)
	require.Equal(t, "auth.test", reg.LastRequest(t).URL.Host)

	requestCount := len(reg.Requests())
	r = httptest.NewRequest(http.MethodPost, "/r/auth", strings.NewReader(strings.Repeat("a", maxAuthRequestBodySize+1)))
	require.Equal(t, http.StatusForbidden, serveRequest(h, r).Code)
	require.Len(t, reg.Requests(), requestCount)
}

func TestAnyProxyAuthRejectsRequestsWithoutPavonisToken(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestAnyHandler(t, reg, true)

	for _, authorization := range []string{"", bearer(testUpstreamToken)} {
		w := serveRequest(h, newRequest(http.MethodHead, "/r/v2/docker.io/library/nginx/manifests/latest", authorization))
		requireChallenge(t, w, map[string]string{"realm": "http://proxy.test/r/auth", "scope": "repository:docker.io/library/nginx:pull"})

		w = serveRequest(h, newRequest(http.MethodGet, "/r/v2/", authorization))
		requireChallenge(t, w, map[string]string{"realm": "http://proxy.test/r/auth"})
	}
	require.Empty(t, reg.Requests())

	require.Equal(t, http.StatusOK, serveRequest(h, newRequest(http.MethodGet, "/r/v2/", bearer(packToken("alice", "pw", "")))).Code)
}

func TestAnyProxyForwardsToRegistryInPath(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestAnyHandler(t, reg, true)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/v2/docker.io/library/nginx/manifests/latest", bearer(packToken("alice", "pw", testUpstreamToken))))
	require.Equal(t, http.StatusOK, w.Code)
	req := reg.LastRequest(t)
	require.Equal(t, "https://registry-1.docker.io/v2/library/nginx/manifests/latest", req.URL.String())
	require.Equal(t, bearer(testUpstreamToken), req.Header.Get("Authorization"))
}

func TestAnyProxyAuthForwardsToUpstreamTokenService(t *testing.T) {
	for _, authEnabled := range []bool{true, false} {
		reg := &fakeRegistry{}
		h := newTestAnyHandler(t, reg, authEnabled)

		w := serveRequest(h, newRequest(http.MethodGet, "/r/auth?scope=repository:ghcr.io/a/b:pull&service=client-service", basicAuth("alice", "pw")))
		require.Equal(t, http.StatusOK, w.Code)
		if authEnabled {
			requirePackedTokenResponse(t, w, testUpstreamToken)
		}

		tokenRequest := reg.LastRequest(t)
		require.Equal(t, "auth.test", tokenRequest.URL.Host)
		require.Equal(t, url.Values{"service": {"svc"}, "scope": {"repository:a/b:pull"}}, tokenRequest.URL.Query())
		require.Empty(t, tokenRequest.Header.Get("Authorization"))
	}
}

func TestAnyProxyDummyTokenForAnonymousUpstream(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"localhost:5000": true}}
	h := newTestAnyHandler(t, reg, false)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/auth?scope=repository:localhost:5000/a/b:pull", ""))
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotEmpty(t, body["token"])

	w = serveRequest(h, newRequest(http.MethodGet, "/r/v2/localhost:5000/a/b/manifests/latest", bearer(body["token"])))
	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, reg.LastRequest(t).Header.Get("Authorization"))
}

func TestAnyProxyRejectsUnsupportedRequests(t *testing.T) {
	reg := &fakeRegistry{}
	authOn, authOff := newTestAnyHandler(t, reg, true), newTestAnyHandler(t, reg, false)
	token := bearer(packToken("alice", "pw", testUpstreamToken))

	for _, tc := range []struct {
		name string
		h    handler.HttpHandler
		r    *http.Request
		want int
	}{
		{"login with auth off", authOff, newRequest(http.MethodGet, "/r/auth?account=alice&service=x", basicAuth("alice", "pw")), http.StatusBadRequest},
		{"upstream credentials", authOn, newRequest(http.MethodGet, "/r/auth?scope=repository:ghcr.io/a/b:pull", basicAuth("alice$u", "pw$p")), http.StatusUnauthorized},
		{"invalid scope", authOn, newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull", basicAuth("alice", "pw")), http.StatusBadRequest},
		{"oauth post", authOn, newRequest(http.MethodPost, "/r/auth", ""), http.StatusMethodNotAllowed},
		{"catalog", authOn, newRequest(http.MethodGet, "/r/v2/_catalog", token), http.StatusNotFound},
		{"tags list", authOn, newRequest(http.MethodGet, "/r/v2/docker.io/library/nginx/tags/list", token), http.StatusNotFound},
		{"no registry", authOn, newRequest(http.MethodGet, "/r/v2/library/nginx/manifests/latest", token), http.StatusBadRequest},
		{"old path format", authOn, newRequest(http.MethodGet, "/r/docker.io/v2/library/nginx/manifests/latest", token), http.StatusNotFound},
	} {
		require.Equal(t, tc.want, serveRequest(tc.h, tc.r).Code, tc.name)
	}
	require.Empty(t, reg.Requests())
}

func TestAnyProxyAuthDropsClientServiceWhenUpstreamHasNone(t *testing.T) {
	reg := &fakeRegistry{withoutService: true}
	h := newTestAnyHandler(t, reg, false)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/auth?scope=repository:ghcr.io/a/b:pull&service=client-service", ""))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, url.Values{"scope": {"repository:a/b:pull"}}, reg.LastRequest(t).URL.Query())
}
