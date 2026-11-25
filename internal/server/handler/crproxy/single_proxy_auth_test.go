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
		require.Equal(t, tc.want, buildChallengeScope(tc.path, tc.method), tc.method+" "+tc.path)
	}
}

func TestSingleProxyAuthRejectsRequestsWithoutPavonisToken(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	for _, authorization := range []string{"", buildTestBearerAuthorization(testUpstreamToken)} {
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
	require.Equal(t, "/v2/", reg.requireLastRequest(t).URL.Path, "the original request must not be forwarded")
}

func TestSingleProxyV1AuthRejectsInvalidCredentials(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
	h := newTestSingleHandler(t, reg, true)

	for name, authorization := range map[string]string{
		"anonymous":                   "",
		"wrong Basic password":        buildTestBasicAuthorization("alice", "bad"),
		"invalid local, upstream set": buildTestBasicAuthorization("alice$upstream", "bad$secret"),
		"upstream Bearer":             buildTestBearerAuthorization(testUpstreamToken),
		"invalid Pavonis Bearer":      buildTestBearerAuthorization(packToken("alice", "bad", testUpstreamToken)),
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
		{"Basic", buildTestBasicAuthorization("alice", "pw"), ""},
		{"Basic with upstream credentials", buildTestBasicAuthorization("alice$upstream", "pw$secret"), buildTestBasicAuthorization("upstream", "secret")},
		{"Pavonis token", buildTestBearerAuthorization(packToken("alice", "pw", "")), ""},
		{"Pavonis token with upstream token", buildTestBearerAuthorization(packToken("alice", "pw", buildTestBearerAuthorization(testUpstreamToken))), buildTestBearerAuthorization(testUpstreamToken)},
	} {
		w := serveRequest(h, newRequest(http.MethodGet, "/r/v1/search?q=nginx", tc.authorization))
		require.Equal(t, http.StatusOK, w.Code, tc.name)
		req := reg.requireLastRequest(t)
		require.Equal(t, "/v1/search?q=nginx", req.URL.RequestURI(), tc.name)
		require.Equal(t, tc.upstreamAuthorization, req.Header.Get("Authorization"), tc.name)
	}
}

func TestSingleProxyV1ForwardsCredentialsWhenAuthDisabled(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
	h := newTestSingleHandler(t, reg, false)

	for _, authorization := range []string{
		buildTestBasicAuthorization("alice$upstream", "pw$secret"),
		buildTestBearerAuthorization(packToken("alice", "pw", buildTestBearerAuthorization(testUpstreamToken))),
	} {
		w := serveRequest(h, newRequest(http.MethodGet, "/r/v1/search?q=nginx", authorization))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, authorization, reg.requireLastRequest(t).Header.Get("Authorization"))
	}
}

func TestSingleProxyAuthForwardsUpstreamTokenInPavonisToken(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/v2/library/nginx/manifests/latest", buildTestBearerAuthorization(packToken("alice", "pw", buildTestBearerAuthorization(testUpstreamToken)))))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, buildTestBearerAuthorization(testUpstreamToken), reg.requireLastRequest(t).Header.Get("Authorization"))

	requestCount := len(reg.Requests())
	w = serveRequest(h, newRequest(http.MethodGet, "/r/v2/", buildTestBearerAuthorization(packToken("alice", "pw", ""))))
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, reg.Requests(), requestCount, "a login-only token is answered by Pavonis")
}

func TestSingleProxyAuthForwardsUpstreamBasicCredentialsInPavonisToken(t *testing.T) {
	reg := &fakeRegistry{basic: map[string]bool{"registry.test": true}}
	h := newTestSingleHandler(t, reg, true)
	upstreamAuthorization := buildTestBasicAuthorization(testUpstreamUser, testUpstreamPassword)

	for _, path := range []string{"/v2/library/nginx/manifests/latest", "/v2/"} {
		w := serveRequest(h, newRequest(http.MethodGet, "/r"+path, buildTestBearerAuthorization(packToken("alice", "pw", upstreamAuthorization))))
		require.Equal(t, http.StatusOK, w.Code, path)
		req := reg.requireLastRequest(t)
		require.Equal(t, path, req.URL.Path)
		require.Equal(t, upstreamAuthorization, req.Header.Get("Authorization"), path)
	}
}

func TestSingleProxyAuthIssuesPavonisTokenForBasicUpstream(t *testing.T) {
	reg := &fakeRegistry{basic: map[string]bool{"registry.test": true}}
	h := newTestSingleHandler(t, reg, true)

	withUpstream := buildTestBasicAuthorization("alice$"+testUpstreamUser, "pw$"+testUpstreamPassword)
	for _, tc := range []struct {
		target, authorization, wantUpstreamAuthorization string
	}{
		{"/r/auth?scope=repository:library/nginx:pull", withUpstream, buildTestBasicAuthorization(testUpstreamUser, testUpstreamPassword)},
		{"/r/auth?scope=repository:library/nginx:pull", buildTestBasicAuthorization("alice", "pw"), ""},
		{"/r/auth", withUpstream, buildTestBasicAuthorization(testUpstreamUser, testUpstreamPassword)},
	} {
		w := serveRequest(h, newRequest(http.MethodGet, tc.target, tc.authorization))
		requirePackedTokenResponse(t, w, tc.wantUpstreamAuthorization)
	}
}

func TestSingleProxyAuthIssuesPavonisToken(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull&service=svc&account=alice", buildTestBasicAuthorization("alice", "pw")))
	requirePackedTokenResponse(t, w, buildTestBearerAuthorization(testUpstreamToken))
	tokenRequest := reg.requireLastRequest(t)
	require.Equal(t, "auth.test", tokenRequest.URL.Host)
	require.False(t, tokenRequest.URL.Query().Has("account"))
	require.Empty(t, tokenRequest.Header.Get("Authorization"))

	w = serveRequest(h, newRequest(http.MethodGet, "/r/auth?account=alice&service=svc", buildTestBasicAuthorization("alice", "pw")))
	requirePackedTokenResponse(t, w, "")
}

func TestSingleProxyAuthForwardsUpstreamCredentialsToTokenService(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull&account=alice%24upuser", buildTestBasicAuthorization("alice$upuser", "pw$upsecret")))
	requirePackedTokenResponse(t, w, buildTestBearerAuthorization(testUpstreamToken))
	tokenRequest := reg.requireLastRequest(t)
	require.Equal(t, "auth.test", tokenRequest.URL.Host)
	require.Equal(t, "upuser", tokenRequest.URL.Query().Get("account"))
	require.Equal(t, buildTestBasicAuthorization("upuser", "upsecret"), tokenRequest.Header.Get("Authorization"))
}

func TestSingleProxyAuthPreservesRealmQuery(t *testing.T) {
	for _, source := range []string{"configured", "discovered"} {
		t.Run(source, func(t *testing.T) {
			realm := "https://auth.test/token?fixed=1"
			reg := &fakeRegistry{authRealm: realm}
			settings := newTestSingleSettings(false)
			if source == "configured" {
				settings.UpstreamAuthRealmUrl = &realm
			}
			h := newSingleHandler(t, reg, settings)

			w := serveRequest(h, newRequest(http.MethodGet, "/r/auth?scope=repository:org/image:pull&service=svc", ""))
			require.Equal(t, http.StatusOK, w.Code)
			tokenRequest := reg.requireLastRequest(t)
			require.Equal(t, "auth.test", tokenRequest.URL.Host)
			require.Equal(t, "/token", tokenRequest.URL.Path)
			require.Equal(t, url.Values{"fixed": {"1"}, "scope": {"repository:org/image:pull"}, "service": {"svc"}}, tokenRequest.URL.Query())
		})
	}
}

func TestSingleProxyAuthRealmForAnonymousUpstream(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}

	w := serveRequest(newTestSingleHandler(t, reg, true), newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull", buildTestBasicAuthorization("alice", "pw")))
	requirePackedTokenResponse(t, w, "")

	w = serveRequest(newTestSingleHandler(t, reg, false), newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull", ""))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSingleProxyAuthRealmForBasicUpstreamWhenAuthDisabled(t *testing.T) {
	reg := &fakeRegistry{basic: map[string]bool{"registry.test": true}}
	w := serveRequest(newTestSingleHandler(t, reg, false), newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull", buildTestBasicAuthorization(testUpstreamUser, testUpstreamPassword)))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSingleProxyAuthRejectsUnsupportedAuthRequests(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	post := newRequest(http.MethodPost, "/r/auth", buildTestBasicAuthorization("alice", "pw"))
	post.Body = http.NoBody
	for _, tc := range []struct {
		name string
		r    *http.Request
		want int
	}{
		{"anonymous", newRequest(http.MethodGet, "/r/auth", ""), http.StatusUnauthorized},
		{"wrong password", newRequest(http.MethodGet, "/r/auth", buildTestBasicAuthorization("alice", "bad")), http.StatusUnauthorized},
		{"oauth post", post, http.StatusUnauthorized},
		{"sub path", newRequest(http.MethodGet, "/r/auth/token", buildTestBasicAuthorization("alice", "pw")), http.StatusNotFound},
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
	require.Equal(t, http.MethodPost, reg.requireLastRequest(t).Method)
	require.Equal(t, "auth.test", reg.requireLastRequest(t).URL.Host)

	requestCount := len(reg.Requests())
	r = httptest.NewRequest(http.MethodPost, "/r/auth", strings.NewReader(strings.Repeat("a", maxAuthRequestBodySize+1)))
	require.Equal(t, http.StatusRequestEntityTooLarge, serveRequest(h, r).Code)
	require.Len(t, reg.Requests(), requestCount)
}
