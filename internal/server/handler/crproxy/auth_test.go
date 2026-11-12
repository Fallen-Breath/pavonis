package crproxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/testutils"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProbeUpstreamChallenge(t *testing.T) {
	for _, tc := range []struct {
		status       int
		authenticate string
		wantScheme   upstreamAuthScheme
		wantParams   map[string]string
		wantErr      bool
	}{
		{status: http.StatusOK, wantScheme: upstreamAuthNone},
		{status: http.StatusUnauthorized, authenticate: `Bearer realm="https://auth.test/token",service="svc"`, wantScheme: upstreamAuthBearer, wantParams: map[string]string{"realm": "https://auth.test/token", "service": "svc"}},
		{status: http.StatusUnauthorized, authenticate: `Basic realm="Registry Realm"`, wantScheme: upstreamAuthBasic},
		{status: http.StatusForbidden, authenticate: `Basic realm="Registry Realm"`, wantErr: true},
	} {
		transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			if tc.authenticate != "" {
				w.Header().Set("Www-Authenticate", tc.authenticate)
			}
			w.WriteHeader(tc.status)
			return w.Result(), nil
		})
		helper := testutils.NewRequestHelperWithTransport(t, transport)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		scheme, params, err := probeUpstreamChallenge(testutils.Context(), helper, r, &url.URL{Scheme: "https", Host: "registry.test", Path: "/v2/"})
		name := fmt.Sprintf("%d %s", tc.status, tc.authenticate)
		if tc.wantErr {
			require.Error(t, err, name)
			continue
		}
		require.NoError(t, err, name)
		require.Equal(t, tc.wantScheme, scheme, name)
		require.Equal(t, tc.wantParams, params, name)
	}
}
