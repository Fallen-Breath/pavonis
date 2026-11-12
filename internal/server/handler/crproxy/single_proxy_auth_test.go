package crproxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChallengeScope(t *testing.T) {
	for _, tc := range []struct {
		path, method, want string
	}{
		{"/v2/library/nginx/manifests/latest", http.MethodHead, "repository:library/nginx:pull"},
		{"/v2/library/nginx/blobs/sha256:abc", http.MethodGet, "repository:library/nginx:pull"},
		{"/v2/a/b/blobs/uploads/", http.MethodPost, "repository:a/b:pull,push"},
		{"/v2/a/b/manifests/v1", http.MethodPut, "repository:a/b:pull,push"},
		{"/v2/", http.MethodGet, ""},
		{"/v2/_catalog", http.MethodGet, ""},
	} {
		require.Equal(t, tc.want, challengeScope(tc.path, tc.method), tc.method+" "+tc.path)
	}
}

func TestSingleProxyAuthRejectsRequestsWithoutPavonisToken(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	for _, authorization := range []string{"", bearer(testUpstreamToken)} {
		w := serveRequest(h, newRequest(http.MethodHead, "/r/v2/library/nginx/manifests/latest", authorization))
		requireChallenge(t, w, map[string]string{"realm": "http://proxy.test/r/auth", "service": "svc", "scope": "repository:library/nginx:pull"})
	}
	for _, req := range reg.Requests() {
		require.Equal(t, "/v2/", req.URL.Path, "the original request must not be forwarded")
		require.Empty(t, req.Header.Get("Authorization"))
	}
}

func TestSingleProxyAuthRejectsRequestsWithoutPavonisTokenForBasicUpstream(t *testing.T) {
	reg := &fakeRegistry{basic: map[string]bool{"registry.test": true}}
	h := newTestSingleHandler(t, reg, true)

	w := serveRequest(h, newRequest(http.MethodHead, "/r/v2/library/nginx/manifests/latest", ""))
	requireChallenge(t, w, map[string]string{"realm": "http://proxy.test/r/auth", "scope": "repository:library/nginx:pull"})
	require.Equal(t, "/v2/", reg.LastRequest(t).URL.Path, "the original request must not be forwarded")
}

func TestSingleProxyV1AuthRejectsInvalidCredentials(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
	h := newTestSingleHandler(t, reg, true)

	for name, authorization := range map[string]string{
		"anonymous":                   "",
		"wrong Basic password":        basicAuth("alice", "bad"),
		"invalid local, upstream set": basicAuth("alice$upstream", "bad$secret"),
		"upstream Bearer":             bearer(testUpstreamToken),
		"invalid Pavonis Bearer":      bearer(packToken("alice", "bad", testUpstreamToken)),
		"legacy Token":                "Token upstream-token",
	} {
		r := newRequest(http.MethodGet, "/r/v1/search?q=nginx", authorization)
		r.Header.Set("X-Docker-Token", "true")
		w := serveRequest(h, r)
		require.Equal(t, http.StatusUnauthorized, w.Code, name)
		require.True(t, strings.HasPrefix(strings.ToLower(w.Header().Get("WWW-Authenticate")), "basic "), "clients must be challenged for Basic credentials")
	}
	require.Empty(t, reg.Requests(), "invalid credentials must not grant access to an anonymous upstream")
}

func TestSingleProxyV1AuthForwardsOnlyUpstreamCredentials(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
	h := newTestSingleHandler(t, reg, true)

	for _, tc := range []struct {
		name, authorization, upstreamAuthorization string
	}{
		{"Basic", basicAuth("alice", "pw"), ""},
		{"Basic with upstream credentials", basicAuth("alice$upstream", "pw$secret"), basicAuth("upstream", "secret")},
		{"Pavonis token", bearer(packToken("alice", "pw", "")), ""},
		{"Pavonis token with upstream token", bearer(packToken("alice", "pw", bearer(testUpstreamToken))), bearer(testUpstreamToken)},
	} {
		w := serveRequest(h, newRequest(http.MethodGet, "/r/v1/search?q=nginx", tc.authorization))
		require.Equal(t, http.StatusOK, w.Code, tc.name)
		req := reg.LastRequest(t)
		require.Equal(t, "/v1/search?q=nginx", req.URL.RequestURI(), tc.name)
		require.Equal(t, tc.upstreamAuthorization, req.Header.Get("Authorization"), tc.name)
	}
}

func TestSingleProxyV1ForwardsCredentialsWhenAuthDisabled(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
	h := newTestSingleHandler(t, reg, false)

	for _, authorization := range []string{
		basicAuth("alice$upstream", "pw$secret"),
		bearer(packToken("alice", "pw", bearer(testUpstreamToken))),
	} {
		w := serveRequest(h, newRequest(http.MethodGet, "/r/v1/search?q=nginx", authorization))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, authorization, reg.LastRequest(t).Header.Get("Authorization"))
	}
}

func TestSingleProxyAuthForwardsUpstreamTokenInPavonisToken(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/v2/library/nginx/manifests/latest", bearer(packToken("alice", "pw", bearer(testUpstreamToken)))))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, bearer(testUpstreamToken), reg.LastRequest(t).Header.Get("Authorization"))

	requestCount := len(reg.Requests())
	w = serveRequest(h, newRequest(http.MethodGet, "/r/v2/", bearer(packToken("alice", "pw", ""))))
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, reg.Requests(), requestCount, "a login-only token is answered by Pavonis")
}

func TestSingleProxyAuthForwardsUpstreamBasicCredentialsInPavonisToken(t *testing.T) {
	reg := &fakeRegistry{basic: map[string]bool{"registry.test": true}}
	h := newTestSingleHandler(t, reg, true)
	upstreamAuthorization := basicAuth(testUpstreamUser, testUpstreamPassword)

	for _, path := range []string{"/v2/library/nginx/manifests/latest", "/v2/"} {
		w := serveRequest(h, newRequest(http.MethodGet, "/r"+path, bearer(packToken("alice", "pw", upstreamAuthorization))))
		require.Equal(t, http.StatusOK, w.Code, path)
		req := reg.LastRequest(t)
		require.Equal(t, path, req.URL.Path)
		require.Equal(t, upstreamAuthorization, req.Header.Get("Authorization"), path)
	}
}

func TestSingleProxyAuthIssuesPavonisTokenForBasicUpstream(t *testing.T) {
	reg := &fakeRegistry{basic: map[string]bool{"registry.test": true}}
	h := newTestSingleHandler(t, reg, true)

	withUpstream := basicAuth("alice$"+testUpstreamUser, "pw$"+testUpstreamPassword)
	for _, tc := range []struct {
		target, authorization, wantUpstreamAuthorization string
	}{
		{"/r/auth?scope=repository:library/nginx:pull", withUpstream, basicAuth(testUpstreamUser, testUpstreamPassword)},
		{"/r/auth?scope=repository:library/nginx:pull", basicAuth("alice", "pw"), ""},
		{"/r/auth", withUpstream, basicAuth(testUpstreamUser, testUpstreamPassword)},
	} {
		w := serveRequest(h, newRequest(http.MethodGet, tc.target, tc.authorization))
		requirePackedTokenResponse(t, w, tc.wantUpstreamAuthorization)
	}
}

func TestSingleProxyAuthIssuesPavonisToken(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull&service=svc&account=alice", basicAuth("alice", "pw")))
	requirePackedTokenResponse(t, w, bearer(testUpstreamToken))
	tokenRequest := reg.LastRequest(t)
	require.Equal(t, "auth.test", tokenRequest.URL.Host)
	require.False(t, tokenRequest.URL.Query().Has("account"))
	require.Empty(t, tokenRequest.Header.Get("Authorization"))

	w = serveRequest(h, newRequest(http.MethodGet, "/r/auth?account=alice&service=svc", basicAuth("alice", "pw")))
	requirePackedTokenResponse(t, w, "")
}

func TestSingleProxyAuthForwardsUpstreamCredentialsToTokenService(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull&account=alice%24upuser", basicAuth("alice$upuser", "pw$upsecret")))
	requirePackedTokenResponse(t, w, bearer(testUpstreamToken))
	tokenRequest := reg.LastRequest(t)
	require.Equal(t, "auth.test", tokenRequest.URL.Host)
	require.Equal(t, "upuser", tokenRequest.URL.Query().Get("account"))
	require.Equal(t, basicAuth("upuser", "upsecret"), tokenRequest.Header.Get("Authorization"))
}

func TestSingleProxyAuthPreservesRealmQuery(t *testing.T) {
	for _, source := range []string{"configured", "discovered"} {
		t.Run(source, func(t *testing.T) {
			realm := "https://auth.test/token?fixed=1"
			reg := &fakeRegistry{authRealm: realm}
			settings := testSingleSettings(false)
			if source == "configured" {
				settings.UpstreamAuthRealmUrl = &realm
			}
			h := newSingleHandler(t, reg, settings)

			w := serveRequest(h, newRequest(http.MethodGet, "/r/auth?scope=repository:org/image:pull&service=svc", ""))
			require.Equal(t, http.StatusOK, w.Code)
			tokenRequest := reg.LastRequest(t)
			require.Equal(t, "auth.test", tokenRequest.URL.Host)
			require.Equal(t, "/token", tokenRequest.URL.Path)
			require.Equal(t, url.Values{"fixed": {"1"}, "scope": {"repository:org/image:pull"}, "service": {"svc"}}, tokenRequest.URL.Query())
		})
	}
}

func TestSingleProxyAuthRealmForAnonymousUpstream(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}

	w := serveRequest(newTestSingleHandler(t, reg, true), newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull", basicAuth("alice", "pw")))
	requirePackedTokenResponse(t, w, "")

	w = serveRequest(newTestSingleHandler(t, reg, false), newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull", ""))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSingleProxyAuthRealmForBasicUpstreamWhenAuthDisabled(t *testing.T) {
	reg := &fakeRegistry{basic: map[string]bool{"registry.test": true}}
	w := serveRequest(newTestSingleHandler(t, reg, false), newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull", basicAuth(testUpstreamUser, testUpstreamPassword)))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSingleProxyAuthRejectsUnsupportedAuthRequests(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	post := newRequest(http.MethodPost, "/r/auth", basicAuth("alice", "pw"))
	post.Body = http.NoBody
	for _, tc := range []struct {
		name string
		r    *http.Request
		want int
	}{
		{"anonymous", newRequest(http.MethodGet, "/r/auth", ""), http.StatusUnauthorized},
		{"wrong password", newRequest(http.MethodGet, "/r/auth", basicAuth("alice", "bad")), http.StatusUnauthorized},
		{"oauth post", post, http.StatusUnauthorized},
		{"sub path", newRequest(http.MethodGet, "/r/auth/token", basicAuth("alice", "pw")), http.StatusNotFound},
	} {
		require.Equal(t, tc.want, serveRequest(h, tc.r).Code, tc.name)
	}
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
	require.Equal(t, http.StatusRequestEntityTooLarge, serveRequest(h, r).Code)
	require.Len(t, reg.Requests(), requestCount)
}
