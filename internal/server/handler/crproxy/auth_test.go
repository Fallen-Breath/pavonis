package crproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBasicAuth(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetBasicAuth("self$upstream", "selfpass$uppass")
	user, pass, selfUser, selfPass, upstreamUser, upstreamPass, ok := parseBasicAuth(r)
	require.True(t, ok)
	require.Equal(t, "self$upstream", user)
	require.Equal(t, "selfpass$uppass", pass)
	require.Equal(t, "self", selfUser)
	require.Equal(t, "selfpass", selfPass)
	require.Equal(t, "upstream", *upstreamUser)
	require.Equal(t, "uppass", *upstreamPass)

	r.SetBasicAuth("plain", "password")
	_, _, selfUser, selfPass, upstreamUser, upstreamPass, ok = parseBasicAuth(r)
	require.True(t, ok)
	require.Equal(t, "plain", selfUser)
	require.Equal(t, "password", selfPass)
	require.Nil(t, upstreamUser)
	require.Nil(t, upstreamPass)
}

func TestParseBasicAuthMissing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, _, _, _, _, _, ok := parseBasicAuth(r)
	require.False(t, ok)
}
