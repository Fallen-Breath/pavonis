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

func TestBearerChallenge(t *testing.T) {
	params, ok := parseBearerChallenge(`Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/nginx:pull"`)
	require.True(t, ok)
	require.Equal(t, map[string]string{"realm": "https://auth.docker.io/token", "service": "registry.docker.io", "scope": "repository:library/nginx:pull"}, params)

	_, ok = parseBearerChallenge(`Basic realm="registry"`)
	require.False(t, ok)

	require.Equal(t, `Bearer realm="https://p/auth"`, buildBearerChallenge("https://p/auth", "", ""))
	params, ok = parseBearerChallenge(buildBearerChallenge("https://p/auth", "svc", "repository:a/b:pull"))
	require.True(t, ok)
	require.Equal(t, map[string]string{"realm": "https://p/auth", "service": "svc", "scope": "repository:a/b:pull"}, params)
}

func TestRewriteAccountQuery(t *testing.T) {
	upstreamUser := "bob"
	for _, tc := range []struct {
		query        string
		upstreamUser *string
		want         string
	}{
		{"account=alice&scope=x", &upstreamUser, "account=bob&scope=x"},
		{"account=alice&scope=x", nil, "scope=x"},
		{"scope=x", &upstreamUser, "scope=x"},
	} {
		r := httptest.NewRequest(http.MethodGet, "/auth?"+tc.query, nil)
		rewriteAccountQuery(r, tc.upstreamUser)
		require.Equal(t, tc.want, r.URL.RawQuery, tc.query)
	}
}
