package crproxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/Fallen-Breath/pavonis/internal/server/auth"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
)

const packedTokenIssuer = "pavonis"

const maxTokenResponseSize = 64 * 1024

// The token issued by Pavonis when auth is enabled. It is not encrypted or signed,
// since the Pavonis credentials inside are checked on every request
type packedToken struct {
	Issuer   string `json:"issuer"`
	User     string `json:"user"`
	Password string `json:"password"`
	// The Authorization header value sent to the upstream, e.g. "Bearer xxx" or "Basic xxx". Empty for none
	UpstreamAuthorization string `json:"upstream_authorization"`
}

func packToken(user, password, upstreamAuthorization string) string {
	data, _ := json.Marshal(packedToken{
		Issuer:                packedTokenIssuer,
		User:                  user,
		Password:              password,
		UpstreamAuthorization: upstreamAuthorization,
	})
	return base64.RawURLEncoding.EncodeToString(data)
}

func unpackToken(s string) (*packedToken, bool) {
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, false
	}
	var token packedToken
	if err := json.Unmarshal(data, &token); err != nil || token.Issuer != packedTokenIssuer {
		return nil, false
	}
	return &token, true
}

// Returns the upstream Authorization inside, if the request carries a Pavonis token with valid credentials
func checkPackedToken(authMgr *auth.UserManager, r *http.Request) (string, bool) {
	s, ok := getBearerToken(r)
	if !ok {
		return "", false
	}
	token, ok := unpackToken(s)
	if !ok || !authMgr.Check(token.User, token.Password) {
		return "", false
	}
	return token.UpstreamAuthorization, true
}

func writeTokenResponse(w http.ResponseWriter, token string) {
	data, _ := json.Marshal(map[string]string{"token": token})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// Prepares an "/auth" request whose successful response will be rewritten by packTokenResponseModifier
func prepareTokenRequest(r *http.Request) {
	// Let the transport handle the compression, so the response body is readable
	r.Header.Del("Accept-Encoding")
}

// Replaces the tokens in a successful token response with packed tokens, and drops the refresh token.
// Docker stores a refresh token as an identity token, and then only refreshes via "POST /auth", which Pavonis rejects
func packTokenResponseModifier(user, password string) common.ResponseModifier {
	return func(_ *http.Request, resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return nil
		}
		if resp.Header.Get("Content-Encoding") != "" {
			return common.NewHttpError(http.StatusBadGateway, "Unsupported token response encoding")
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseSize+1))
		_ = resp.Body.Close()
		if err != nil {
			return err
		}
		if len(data) > maxTokenResponseSize {
			return common.NewHttpError(http.StatusBadGateway, "Token response too large")
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			return common.NewHttpError(http.StatusBadGateway, "Invalid token response")
		}
		for _, key := range []string{"token", "access_token"} {
			if raw, ok := fields[key]; ok {
				var upstreamToken string
				if err := json.Unmarshal(raw, &upstreamToken); err != nil {
					return common.NewHttpError(http.StatusBadGateway, "Invalid token response")
				}
				fields[key], _ = json.Marshal(packToken(user, password, bearerAuthorization(upstreamToken)))
			}
		}
		delete(fields, "refresh_token")

		data, _ = json.Marshal(fields)
		resp.Body = io.NopCloser(bytes.NewReader(data))
		resp.ContentLength = int64(len(data))
		resp.Header.Set("Content-Length", strconv.Itoa(len(data)))
		return nil
	}
}
