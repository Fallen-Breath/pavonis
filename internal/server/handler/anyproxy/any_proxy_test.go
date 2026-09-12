package anyproxy

import (
	"net/http"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/stretchr/testify/require"
)

func TestIsAllowedMethod(t *testing.T) {
	h := &proxyHandler{settings: &config.AnyProxySettings{}}
	require.True(t, h.isAllowedMethod(http.MethodGet))
	require.False(t, h.isAllowedMethod(http.MethodPost))
	h.settings.AllowedMethods = []string{"get", "POST"}
	require.True(t, h.isAllowedMethod(http.MethodGet))
	require.True(t, h.isAllowedMethod(http.MethodPost))
	require.False(t, h.isAllowedMethod(http.MethodDelete))
}

func TestIsBlacklisted(t *testing.T) {
	h := &proxyHandler{settings: &config.AnyProxySettings{DomainBlacklist: []string{
		"example.com", "*.blocked.test", "  ignored.example. ",
	}}}
	require.True(t, h.isBlacklisted("EXAMPLE.COM."))
	require.True(t, h.isBlacklisted("a.blocked.test"))
	require.False(t, h.isBlacklisted("blocked.test"))
	require.True(t, h.isBlacklisted("ignored.example"))
	require.False(t, h.isBlacklisted("other.test"))
}
