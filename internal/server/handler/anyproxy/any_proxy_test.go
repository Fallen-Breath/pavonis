package anyproxy

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func newTestProxyHandler(t *testing.T, settings *config.AnyProxySettings) *proxyHandler {
	h, err := NewHandler(testutils.SiteInfo("any", config.SiteModeAnyProxy, "/proxy", "http://proxy.test"), testutils.NewRequestHelper(t), settings)
	require.NoError(t, err)
	t.Cleanup(h.Shutdown)
	return h.(*proxyHandler)
}

func TestIsAllowedMethod(t *testing.T) {
	h := newTestProxyHandler(t, &config.AnyProxySettings{AllowedMethods: []string{"get", "POST"}})
	require.True(t, h.isAllowedMethod(http.MethodGet))
	require.True(t, h.isAllowedMethod(http.MethodPost))
	require.False(t, h.isAllowedMethod(http.MethodDelete))
}

func TestResolveTargetAccepts(t *testing.T) {
	for _, tc := range []struct {
		name, reqPath, rawQuery, expected string
	}{
		{"https", "/https://example.com/a", "", "https://example.com/a"},
		{"http", "/http://example.com/a", "", "http://example.com/a"},
		{"without scheme", "/example.com/a", "", "https://example.com/a"},
		{"without scheme, with port", "/example.com:8443/a", "", "https://example.com:8443/a"},
		{"scheme-like text in path", "/example.com/path/file://name", "", "https://example.com/path/file://name"},
		{"query", "/https://example.com/a", "x=1", "https://example.com/a?x=1"},
		{"userinfo kept", "/http://a:b@example.com/a", "", "http://a:b@example.com/a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, httpErr := newTestProxyHandler(t, &config.AnyProxySettings{}).resolveTarget(tc.reqPath, tc.rawQuery)
			require.Nil(t, httpErr)
			require.Equal(t, tc.expected, u.String())
		})
	}
}

func TestResolveTargetRejects(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings config.AnyProxySettings
		reqPath  string
		status   int
	}{
		{"empty", config.AnyProxySettings{}, "/", http.StatusBadRequest},
		{"protocol relative", config.AnyProxySettings{}, "///example.com/a", http.StatusBadRequest},
		{"unsupported scheme", config.AnyProxySettings{}, "/ftp://example.com/a", http.StatusBadRequest},
		{"not whitelisted", config.AnyProxySettings{DomainWhitelist: []string{"example.org"}}, "/example.com/a", http.StatusForbidden},
		{"blacklisted", config.AnyProxySettings{DomainBlacklist: []string{"example.com"}}, "/example.com/a", http.StatusForbidden},
		{"blacklist wins over whitelist", config.AnyProxySettings{DomainWhitelist: []string{"example.com"}, DomainBlacklist: []string{"example.com"}}, "/example.com/a", http.StatusForbidden},
		{"private ip", config.AnyProxySettings{}, "/http://127.0.0.1/a", http.StatusForbidden},
		{"ipv4-mapped private ip", config.AnyProxySettings{}, "/http://[::ffff:127.0.0.1]/a", http.StatusForbidden},
		{"link local ip", config.AnyProxySettings{}, "/http://169.254.169.254/a", http.StatusForbidden},
		{"localhost", config.AnyProxySettings{}, "/http://localhost/a", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, httpErr := newTestProxyHandler(t, &tc.settings).resolveTarget(tc.reqPath, "")
			require.Nil(t, u)
			require.NotNil(t, httpErr)
			require.Equal(t, tc.status, httpErr.Status)
		})
	}
}

func TestResolveTargetAllowsPrivateNetwork(t *testing.T) {
	h := newTestProxyHandler(t, &config.AnyProxySettings{AllowPrivateNetwork: true})
	for _, reqPath := range []string{"/http://127.0.0.1/a", "/http://localhost/a"} {
		_, httpErr := h.resolveTarget(reqPath, "")
		require.Nil(t, httpErr, reqPath)
	}
}

func basicAuth(user, password string) string {
	r := &http.Request{Header: make(http.Header)}
	r.SetBasicAuth(user, password)
	return r.Header.Get("Authorization")
}

func TestSetUpstreamAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name           string
		target         string
		upstreamHeader string
		expected       string
	}{
		{"none", "https://example.com/a", "", ""},
		{"upstream header", "https://example.com/a", "Bearer token", "Bearer token"},
		{"url userinfo", "https://a:b@example.com/a", "", basicAuth("a", "b")},
		{"url username only", "https://a@example.com/a", "", basicAuth("a", "")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target, err := url.Parse(tc.target)
			require.NoError(t, err)
			r := &http.Request{Header: make(http.Header)}
			r.SetBasicAuth("pavonis-user", "pavonis-password")
			if tc.upstreamHeader != "" {
				r.Header.Set(headerUpstreamAuthorization, tc.upstreamHeader)
			}

			require.NoError(t, setUpstreamAuthorization(r, target))
			require.Equal(t, tc.expected, r.Header.Get("Authorization"))
			require.Empty(t, r.Header.Values(headerUpstreamAuthorization))
			require.Nil(t, target.User)
		})
	}

	t.Run("both url and header", func(t *testing.T) {
		target, err := url.Parse("https://a:b@example.com/a")
		require.NoError(t, err)
		r := &http.Request{Header: make(http.Header)}
		r.Header.Set(headerUpstreamAuthorization, "Bearer token")
		require.Error(t, setUpstreamAuthorization(r, target))
	})
}

func TestHandleRedirect(t *testing.T) {
	newResp := func(location string) *http.Response {
		req, _ := http.NewRequest(http.MethodGet, "https://example.com/start", nil)
		resp := &http.Response{StatusCode: http.StatusFound, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: req}
		if location != "" {
			resp.Header.Set("Location", location)
		}
		return resp
	}
	h := newTestProxyHandler(t, &config.AnyProxySettings{DomainBlacklist: []string{"blocked.example"}})

	for _, tc := range []struct {
		name     string
		location string
		expected common.RedirectDecision
	}{
		{"no location", "", common.RedirectDecisionReturn},
		{"relative", "/final", common.RedirectDecisionFollow},
		{"allowed host", "https://other.example/final", common.RedirectDecisionFollow},
		{"blacklisted host", "https://blocked.example/final", common.RedirectDecisionAbort},
		{"private host", "http://127.0.0.1/final", common.RedirectDecisionAbort},
		{"unsupported scheme", "ftp://other.example/final", common.RedirectDecisionAbort},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := h.handleRedirect(newResp(tc.location))
			require.Equal(t, tc.expected, result.Decision)
			if tc.expected == common.RedirectDecisionAbort {
				var httpErr *common.HttpError
				require.ErrorAs(t, result.Err, &httpErr)
				require.Equal(t, http.StatusForbidden, httpErr.Status)
			}
		})
	}
}

func TestModifyResponseRenamesUpstreamChallenge(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Add("WWW-Authenticate", `Basic realm="a"`)
	resp.Header.Add("WWW-Authenticate", `Bearer realm="b"`)
	require.NoError(t, newTestProxyHandler(t, &config.AnyProxySettings{}).modifyResponse(nil, resp))
	require.Empty(t, resp.Header.Values("WWW-Authenticate"))
	require.Equal(t, []string{`Basic realm="a"`, `Bearer realm="b"`}, resp.Header.Values(headerUpstreamWwwAuthenticate))
}
