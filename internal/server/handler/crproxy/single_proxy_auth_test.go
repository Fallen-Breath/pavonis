package crproxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	servercontext "github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
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

	for _, authorization := range []string{"", bearer(testUpstreamToken), bearer(packToken("alice", "bad", testUpstreamToken))} {
		w := serveRequest(h, newRequest(http.MethodHead, "/r/v2/library/nginx/manifests/latest", authorization))
		requireChallenge(t, w, map[string]string{"realm": "http://proxy.test/r/auth", "service": "svc", "scope": "repository:library/nginx:pull"})
	}
	for _, req := range reg.Requests() {
		require.Equal(t, "/v2/", req.URL.Path, "the original request must not be forwarded")
		require.Empty(t, req.Header.Get("Authorization"))
	}
}

func TestSingleProxyV1AuthRejectsInvalidCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, authorization string
	}{
		{"anonymous", ""},
		{"wrong Basic password", basicAuth("alice", "bad")},
		{"unknown Basic user", basicAuth("bob", "pw")},
		{"invalid local credentials with upstream credentials", basicAuth("alice$upstream", "bad$secret")},
		{"malformed Basic", "Basic !!!"},
		{"upstream Bearer", bearer(testUpstreamToken)},
		{"invalid Pavonis Bearer", bearer(packToken("alice", "bad", testUpstreamToken))},
		{"legacy Token", "Token upstream-token"},
		{"unsupported scheme", "Digest credentials"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
			h := newTestSingleHandler(t, reg, true)

			for _, path := range []string{"/r/v1/_ping", "/r/v1/search?q=nginx", "/r/v1/repositories/library/nginx/tags"} {
				for _, method := range []string{http.MethodGet, http.MethodHead} {
					r := newRequest(method, path, tc.authorization)
					r.Header.Set("X-Docker-Token", "true")
					w := serveRequest(h, r)
					require.Equal(t, http.StatusUnauthorized, w.Code, method+" "+path)
					require.True(t, strings.HasPrefix(strings.ToLower(w.Header().Get("WWW-Authenticate")), "basic "), "clients must be challenged for Basic credentials")
					require.Empty(t, reg.Requests(), "invalid credentials must not grant access to an anonymous upstream")
				}
			}
		})
	}
}

func TestSingleProxyV1AuthIsolatesBasicCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, authorization, upstreamAuthorization string
	}{
		{"Pavonis only", basicAuth("alice", "pw"), ""},
		{"with upstream credentials", basicAuth("alice$upstream", "pw$secret"), basicAuth("upstream", "secret")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
			h := newTestSingleHandler(t, reg, true)

			for _, path := range []string{"/v1/_ping", "/v1/search?q=nginx", "/v1/repositories/library/nginx/tags"} {
				for _, method := range []string{http.MethodGet, http.MethodHead} {
					w := serveRequest(h, newRequest(method, "/r"+path, tc.authorization))
					require.Equal(t, http.StatusOK, w.Code, method+" "+path)
					req := reg.LastRequest(t)
					require.Equal(t, method, req.Method)
					require.Equal(t, path, req.URL.RequestURI())
					require.Equal(t, tc.upstreamAuthorization, req.Header.Get("Authorization"))
				}
			}
		})
	}
}

func TestSingleProxyV1AuthIsolatesBearerCredentials(t *testing.T) {
	for _, upstreamToken := range []string{"", testUpstreamToken} {
		reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
		h := newTestSingleHandler(t, reg, true)
		authorization := bearer(packToken("alice", "pw", upstreamToken))
		wantAuthorization := ""
		if upstreamToken != "" {
			wantAuthorization = bearer(upstreamToken)
		}

		for _, path := range []string{"/v1/_ping", "/v1/search?q=nginx", "/v1/repositories/library/nginx/tags"} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				w := serveRequest(h, newRequest(method, "/r"+path, authorization))
				require.Equal(t, http.StatusOK, w.Code, method+" "+path)
				req := reg.LastRequest(t)
				require.Equal(t, method, req.Method)
				require.Equal(t, path, req.URL.RequestURI())
				require.Equal(t, wantAuthorization, req.Header.Get("Authorization"))
			}
		}
	}
}

func TestSingleProxyV1ForwardsCredentialsWhenAuthDisabled(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
	h := newTestSingleHandler(t, reg, false)

	for _, authorization := range []string{
		"",
		basicAuth("upstream", "secret"),
		basicAuth("alice$upstream", "pw$secret"),
		bearer(testUpstreamToken),
		bearer(packToken("alice", "pw", testUpstreamToken)),
		"Token upstream-token",
	} {
		for _, path := range []string{"/v1/_ping", "/v1/search?q=nginx", "/v1/repositories/library/nginx/tags"} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				w := serveRequest(h, newRequest(method, "/r"+path, authorization))
				require.Equal(t, http.StatusOK, w.Code, method+" "+path)
				req := reg.LastRequest(t)
				require.Equal(t, method, req.Method)
				require.Equal(t, path, req.URL.RequestURI())
				require.Equal(t, authorization, req.Header.Get("Authorization"))
			}
		}
	}
}

func TestSingleProxyAuthForwardsUpstreamTokenInPavonisToken(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/v2/library/nginx/manifests/latest", bearer(packToken("alice", "pw", testUpstreamToken))))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, bearer(testUpstreamToken), reg.LastRequest(t).Header.Get("Authorization"))

	requestCount := len(reg.Requests())
	w = serveRequest(h, newRequest(http.MethodGet, "/r/v2/", bearer(packToken("alice", "pw", ""))))
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, reg.Requests(), requestCount, "a login-only token is answered by Pavonis")
}

func TestSingleProxyAuthIssuesPavonisToken(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	w := serveRequest(h, newRequest(http.MethodGet, "/r/auth?scope=repository:library/nginx:pull&service=svc&account=alice", basicAuth("alice", "pw")))
	requirePackedTokenResponse(t, w, testUpstreamToken)
	tokenRequest := reg.LastRequest(t)
	require.Equal(t, "auth.test", tokenRequest.URL.Host)
	require.False(t, tokenRequest.URL.Query().Has("account"))
	require.Empty(t, tokenRequest.Header.Get("Authorization"))

	w = serveRequest(h, newRequest(http.MethodGet, "/r/auth?account=alice&service=svc", basicAuth("alice", "pw")))
	requirePackedTokenResponse(t, w, "")
}

func TestSingleProxyAuthPreservesRealmQuery(t *testing.T) {
	for _, source := range []string{"configured", "discovered"} {
		t.Run(source, func(t *testing.T) {
			realm := "https://auth.test/token?fixed=1"
			reg := &fakeRegistry{authRealm: realm}
			settings := &config.ContainerRegistrySingleProxySettings{
				UpstreamV2Url: strPtr("https://registry.test/v2"),
				Auth:          testAuthConfig(false),
				AllowPush:     boolPtr(false),
				AllowList:     boolPtr(true),
			}
			if source == "configured" {
				settings.UpstreamAuthRealmUrl = &realm
			}
			info := testutils.SiteInfo("cr", config.SiteModeContainerRegistrySingleProxy, "/r", "http://proxy.test")
			h, err := NewContainerRegistrySingleProxyHandler(info, testutils.NewRequestHelperWithTransport(t, reg), settings)
			require.NoError(t, err)
			t.Cleanup(h.Shutdown)

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

func TestSingleProxyAuthRejectsUnsupportedAuthRequests(t *testing.T) {
	reg := &fakeRegistry{}
	h := newTestSingleHandler(t, reg, true)

	r := newRequest(http.MethodPost, "/r/auth", basicAuth("alice", "pw"))
	r.Body = http.NoBody
	require.Equal(t, http.StatusUnauthorized, serveRequest(h, r).Code)
	require.Equal(t, http.StatusNotFound, serveRequest(h, newRequest(http.MethodGet, "/r/auth/token", basicAuth("alice", "pw"))).Code)
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
	require.Equal(t, http.StatusForbidden, serveRequest(h, r).Code)
	require.Len(t, reg.Requests(), requestCount)
}

func TestSingleProxyRewritesAuthRealm(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Www-Authenticate", `Bearer realm="http://`+r.Host+`/token",service="registry"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer up.Close()

	helper := testutils.NewRequestHelper(t)
	mode := config.SiteModeContainerRegistrySingleProxy
	site := &config.SiteConfig{Id: "cr", Mode: &mode, Host: config.SiteHosts{"*"}, PathPrefix: "/r", SelfUrl: "http://proxy.test"}
	info := handler.NewSiteInfo(site.Id, site)
	h, err := NewContainerRegistrySingleProxyHandler(info, helper, &config.ContainerRegistrySingleProxySettings{
		UpstreamV2Url: strPtr(up.URL),
		Auth:          &config.ClientAuthConfig{},
		AllowPush:     boolPtr(true),
		AllowList:     boolPtr(true),
	})
	require.NoError(t, err)
	defer h.Shutdown()

	r := httptest.NewRequest(http.MethodGet, "/r/v2/library/alpine/manifests/latest", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(servercontext.NewRequestContext("x", "127.0.0.1"), w, r)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, `Bearer realm="http://proxy.test/r/auth",service="registry"`, w.Header().Get("Www-Authenticate"))
}

func TestSingleProxyAuthRealmRejectsInvalidCredentials(t *testing.T) {
	helper := testutils.NewRequestHelper(t)
	mode := config.SiteModeContainerRegistrySingleProxy
	site := &config.SiteConfig{Id: "cr-auth-invalid", Mode: &mode, Host: config.SiteHosts{"*"}, PathPrefix: "/r", SelfUrl: "http://proxy.test"}
	info := handler.NewSiteInfo(site.Id, site)
	auth := &config.ClientAuthConfig{Enabled: true, Users: []*config.User{{Name: "user", Password: "pass"}}}
	realm := "http://registry.example/token"
	h, err := NewContainerRegistrySingleProxyHandler(info, helper, &config.ContainerRegistrySingleProxySettings{UpstreamAuthRealmUrl: &realm, UpstreamV2Url: strPtr("http://127.0.0.1:1"), Auth: auth, AllowPush: boolPtr(false), AllowList: boolPtr(true)})
	require.NoError(t, err)
	defer h.Shutdown()

	r := httptest.NewRequest(http.MethodGet, "/r/auth", nil)
	r.SetBasicAuth("user", "wrong")
	w := httptest.NewRecorder()
	h.ServeHttp(servercontext.NewRequestContext("x", "127.0.0.1"), w, r)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSingleProxyAuthRealmForwardsUpstreamCredentials(t *testing.T) {
	var gotUser, gotPassword, gotAccount string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPassword, _ = r.BasicAuth()
		gotAccount = r.URL.Query().Get("account")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"upstream"}`))
	}))
	defer up.Close()

	helper := testutils.NewRequestHelper(t)
	mode := config.SiteModeContainerRegistrySingleProxy
	site := &config.SiteConfig{Id: "cr-auth-upstream", Mode: &mode, Host: config.SiteHosts{"*"}, PathPrefix: "/r", SelfUrl: "http://proxy.test"}
	info := handler.NewSiteInfo(site.Id, site)
	auth := &config.ClientAuthConfig{Enabled: true, Users: []*config.User{{Name: "self", Password: "selfpass"}}}
	realm := up.URL + "/token"
	h, err := NewContainerRegistrySingleProxyHandler(info, helper, &config.ContainerRegistrySingleProxySettings{UpstreamAuthRealmUrl: &realm, UpstreamV2Url: strPtr(up.URL), Auth: auth, AllowPush: boolPtr(true), AllowList: boolPtr(true)})
	require.NoError(t, err)
	defer h.Shutdown()

	r := httptest.NewRequest(http.MethodGet, "/r/auth?scope=repository:library/alpine:pull&account=self%24upstream", nil)
	r.SetBasicAuth("self$upstream", "selfpass$uppass")
	w := httptest.NewRecorder()
	h.ServeHttp(servercontext.NewRequestContext("x", "127.0.0.1"), w, r)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "upstream", gotUser)
	require.Equal(t, "uppass", gotPassword)
	require.Equal(t, "upstream", gotAccount)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	token, ok := unpackToken(body["token"])
	require.True(t, ok)
	require.Equal(t, packedToken{Issuer: packedTokenIssuer, User: "self", Password: "selfpass", UpstreamToken: "upstream"}, *token)
}
