package crproxy

import (
	"net/http"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestIsRegistryHost(t *testing.T) {
	for _, s := range []string{"docker.io", "ghcr.io", "localhost", "localhost:5000", "registry:5000", "10.0.0.1:5000"} {
		require.True(t, isRegistryHost(s), s)
	}
	for _, s := range []string{"library", "nginx", "_catalog", "", "-bad.io", "bad_host.io"} {
		require.False(t, isRegistryHost(s), s)
	}
}

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

func TestAnyProxyReposListTreatsDockerHubAliasesAsEqual(t *testing.T) {
	for _, entry := range []string{"docker.io/library", "registry-1.docker.io/library"} {
		list := newRegistryReposList([]string{entry})
		for _, registry := range []string{"docker.io", "registry-1.docker.io"} {
			require.True(t, list.Check([]string{normalizeRegistry(registry), "library", "nginx"}), entry+" vs "+registry)
		}
		require.False(t, list.Check([]string{normalizeRegistry("ghcr.io"), "library", "nginx"}), entry)
	}
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

func TestAnyProxyRejectsUnrecognizedRepositoryPaths(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestAnyHandler(t, reg, false)
	for _, path := range []string{
		"/r/v2/registry.test/org//image/manifests/latest",
		"/r/v2/registry.test/org/image/manifests/latest/.",
		"/r/v2/registry.test/open/../org/image/manifests/latest",
		"/r/v2/registry.test/org/image/referrers/sha256:abc123",
		"/r/v2/registry.test/_catalog",
	} {
		w := serveRequest(h, newRequest(http.MethodGet, path, ""))
		require.Equal(t, http.StatusNotFound, w.Code, path)
	}
	require.Empty(t, reg.Requests())
}
