package crproxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	authRealm      string

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
		realm := f.authRealm
		if realm == "" {
			realm = "https://auth.test/token"
		}
		w.Header().Set("Www-Authenticate", `Bearer realm="`+realm+`",service="svc"`)
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
