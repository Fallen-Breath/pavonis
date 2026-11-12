package crproxy

import (
	"net/http"
	"testing"

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

func TestAnyProxyReposListTreatsDockerHubAliasesAsEqual(t *testing.T) {
	for _, entry := range []string{"docker.io/library", "registry-1.docker.io/library"} {
		list := newRegistryReposList([]string{entry})
		for _, registry := range []string{"docker.io", "registry-1.docker.io"} {
			require.True(t, list.Check([]string{normalizeRegistry(registry), "library", "nginx"}), entry+" vs "+registry)
		}
		require.False(t, list.Check([]string{normalizeRegistry("ghcr.io"), "library", "nginx"}), entry)
	}
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
