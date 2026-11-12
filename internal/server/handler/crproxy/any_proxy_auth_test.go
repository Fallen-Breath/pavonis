package crproxy

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestParseAuthScopes(t *testing.T) {
	registry, stripped, ok := parseAuthScopes([]string{"repository:docker.io/library/nginx:pull"})
	require.True(t, ok)
	require.Equal(t, "docker.io", registry)
	require.Equal(t, []string{"repository:library/nginx:pull"}, stripped)

	registry, stripped, ok = parseAuthScopes([]string{"repository:localhost:5000/a/b:pull", "repository:localhost:5000/c:pull,push"})
	require.True(t, ok)
	require.Equal(t, "localhost:5000", registry)
	require.Equal(t, []string{"repository:a/b:pull", "repository:c:pull,push"}, stripped)

	for name, scopes := range map[string][]string{
		"mixed registries":   {"repository:docker.io/library/nginx:pull", "repository:ghcr.io/a/b:pull"},
		"no registry":        {"repository:library/nginx:pull"},
		"non-repository":     {"registry:catalog:*"},
		"empty repository":   {"repository:docker.io/:pull"},
		"registry only":      {"repository:docker.io:pull"},
		"missing actions":    {"repository"},
		"one bad among good": {"repository:docker.io/library/nginx:pull", "registry:catalog:*"},
	} {
		_, _, ok := parseAuthScopes(scopes)
		require.False(t, ok, name)
	}
}

func TestPrefixScopesWithRegistry(t *testing.T) {
	require.Equal(t, "repository:docker.io/library/nginx:pull", prefixScopesWithRegistry("repository:library/nginx:pull", "docker.io"))
	require.Equal(t, "repository:ghcr.io/a/b:pull registry:catalog:*", prefixScopesWithRegistry("repository:a/b:pull registry:catalog:*", "ghcr.io"))
	require.Equal(t, "", prefixScopesWithRegistry("", "ghcr.io"))
}

func TestAnyProxyV2ResponseModifier(t *testing.T) {
	h := &anyProxyHandler{info: testutils.SiteInfo("cra", config.SiteModeContainerRegistryAnyProxy, "/r", "http://proxy.test")}
	upstreamChallenge := `Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/nginx:pull"`
	newResp := func(status int) *http.Response {
		resp := &http.Response{StatusCode: status, Header: make(http.Header)}
		resp.Header.Set("Www-Authenticate", upstreamChallenge)
		return resp
	}

	resp := newResp(http.StatusUnauthorized)
	require.NoError(t, h.createV2ResponseModifier("docker.io", false)(nil, resp))
	params, ok := parseBearerChallenge(resp.Header.Get("Www-Authenticate"))
	require.True(t, ok)
	require.Equal(t, map[string]string{
		"realm":   "http://proxy.test/r/auth",
		"service": "registry.docker.io",
		"scope":   "repository:docker.io/library/nginx:pull",
	}, params)

	resp = newResp(http.StatusUnauthorized)
	require.NoError(t, h.createV2ResponseModifier("docker.io", true)(nil, resp))
	require.Empty(t, resp.Header.Values("Www-Authenticate"))

	resp = newResp(http.StatusOK)
	require.NoError(t, h.createV2ResponseModifier("docker.io", false)(nil, resp))
	require.Equal(t, upstreamChallenge, resp.Header.Get("Www-Authenticate"))
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

	w := serveRequest(h, newRequest(http.MethodGet, "/r/v2/docker.io/library/nginx/manifests/latest", bearer(packToken("alice", "pw", bearer(testUpstreamToken)))))
	require.Equal(t, http.StatusOK, w.Code)
	req := reg.LastRequest(t)
	require.Equal(t, "https://registry-1.docker.io/v2/library/nginx/manifests/latest", req.URL.String())
	require.Equal(t, bearer(testUpstreamToken), req.Header.Get("Authorization"))
}

func TestAnyProxyAuthForwardsToUpstreamTokenService(t *testing.T) {
	for _, authEnabled := range []bool{true, false} {
		reg := &fakeRegistry{authRealm: "https://auth.test/token?fixed=1"}
		h := newTestAnyHandler(t, reg, authEnabled)

		w := serveRequest(h, newRequest(http.MethodGet, "/r/auth?scope=repository:ghcr.io/a/b:pull&service=client-service", basicAuth("alice", "pw")))
		require.Equal(t, http.StatusOK, w.Code)
		if authEnabled {
			requirePackedTokenResponse(t, w, bearer(testUpstreamToken))
		}

		tokenRequest := reg.LastRequest(t)
		require.Equal(t, "auth.test", tokenRequest.URL.Host)
		require.Equal(t, url.Values{"fixed": {"1"}, "service": {"svc"}, "scope": {"repository:a/b:pull"}}, tokenRequest.URL.Query())
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

func TestAnyProxyAuthRejectsBasicUpstream(t *testing.T) {
	reg := &fakeRegistry{basic: map[string]bool{"localhost:5000": true}}
	for _, authEnabled := range []bool{true, false} {
		authorization := ""
		if authEnabled {
			authorization = basicAuth("alice", "pw")
		}
		w := serveRequest(newTestAnyHandler(t, reg, authEnabled), newRequest(http.MethodGet, "/r/auth?scope=repository:localhost:5000/a/b:pull", authorization))
		require.Equal(t, http.StatusNotImplemented, w.Code, "auth enabled: %v", authEnabled)
	}
}

func TestAnyProxyRejectsUnsupportedRequests(t *testing.T) {
	reg := &fakeRegistry{}
	authOn, authOff := newTestAnyHandler(t, reg, true), newTestAnyHandler(t, reg, false)
	token := bearer(packToken("alice", "pw", bearer(testUpstreamToken)))

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
		{"unrecognized repository path", authOn, newRequest(http.MethodGet, "/r/v2/docker.io/library/nginx/referrers/sha256:abc", token), http.StatusNotFound},
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
