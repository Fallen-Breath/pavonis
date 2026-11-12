package crproxy

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/auth"
	"github.com/stretchr/testify/require"
)

func newTestUserManager(t *testing.T) *auth.UserManager {
	authMgr, err := auth.NewUserManager("test", &config.ClientAuthConfig{Enabled: true, Users: []*config.User{{Name: "alice", Password: "pw"}}})
	require.NoError(t, err)
	t.Cleanup(authMgr.Shutdown)
	return authMgr
}

func TestPackTokenRoundTrip(t *testing.T) {
	token, ok := unpackToken(packToken("alice", "p$w:d", "upstream.jwt"))
	require.True(t, ok)
	require.Equal(t, "alice", token.User)
	require.Equal(t, "p$w:d", token.Password)
	require.Equal(t, "upstream.jwt", token.UpstreamToken)
}

func TestUnpackTokenRejectsForeignTokens(t *testing.T) {
	encode := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	for _, s := range []string{
		"",
		"not base64!",
		encode("not json"),
		encode(`{"user":"alice","password":"pw"}`),
		encode(`{"issuer":"other","user":"alice","password":"pw"}`),
		"eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJhdXRoLmRvY2tlci5pbyJ9.c2ln",
	} {
		_, ok := unpackToken(s)
		require.False(t, ok, s)
	}
}

func TestCheckPackedToken(t *testing.T) {
	authMgr := newTestUserManager(t)
	request := func(authorization string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if authorization != "" {
			r.Header.Set("Authorization", authorization)
		}
		return r
	}

	upstreamToken, ok := checkPackedToken(authMgr, request("Bearer "+packToken("alice", "pw", "up")))
	require.True(t, ok)
	require.Equal(t, "up", upstreamToken)

	for name, authorization := range map[string]string{
		"missing":        "",
		"wrong password": "Bearer " + packToken("alice", "bad", "up"),
		"unknown user":   "Bearer " + packToken("bob", "pw", "up"),
		"basic":          "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:pw")),
		"upstream token": "Bearer eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJhdXRoLmRvY2tlci5pbyJ9.c2ln",
	} {
		_, ok := checkPackedToken(authMgr, request(authorization))
		require.False(t, ok, name)
	}
}

func tokenResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestPackTokenResponseModifier(t *testing.T) {
	modifier := packTokenResponseModifier("alice", "pw")

	resp := tokenResponse(http.StatusOK, `{"token":"up","access_token":"up","refresh_token":"r","expires_in":300,"issued_at":"2026-01-01T00:00:00Z"}`)
	require.NoError(t, modifier(nil, resp))
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(data, &body))

	require.NotContains(t, body, "refresh_token")
	require.EqualValues(t, 300, body["expires_in"])
	require.Equal(t, "2026-01-01T00:00:00Z", body["issued_at"])
	for _, key := range []string{"token", "access_token"} {
		token, ok := unpackToken(body[key].(string))
		require.True(t, ok, key)
		require.Equal(t, packedToken{Issuer: packedTokenIssuer, User: "alice", Password: "pw", UpstreamToken: "up"}, *token, key)
	}
	require.EqualValues(t, len(data), resp.ContentLength)
}

func TestPackTokenResponseModifierKeepsFailedResponses(t *testing.T) {
	resp := tokenResponse(http.StatusUnauthorized, `{"details":"incorrect username or password"}`)
	require.NoError(t, packTokenResponseModifier("alice", "pw")(nil, resp))
	data, _ := io.ReadAll(resp.Body)
	require.Equal(t, `{"details":"incorrect username or password"}`, string(data))
}

func TestPackTokenResponseModifierRejectsBadResponses(t *testing.T) {
	for name, body := range map[string]string{
		"not json":         "<html></html>",
		"non-string token": `{"token":123}`,
		"too large":        `{"token":"up"}` + strings.Repeat(" ", maxTokenResponseSize),
	} {
		require.Error(t, packTokenResponseModifier("alice", "pw")(nil, tokenResponse(http.StatusOK, body)), name)
	}
}
