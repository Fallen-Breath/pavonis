package crproxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/server/auth"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
)

const packedTokenIssuer = "pavonis"
const maxTokenResponseSize = 64 * 1024
const maxAuthRequestBodySize = 64 * 1024

// The token issued by Pavonis when auth is enabled. It is not encrypted or signed,
// since the Pavonis credentials inside are checked on every request
type packedToken struct {
	Issuer        string `json:"issuer"`
	User          string `json:"user"`
	Password      string `json:"password"`
	UpstreamToken string `json:"upstream_token"`
}

func packToken(user, password, upstreamToken string) string {
	data, _ := json.Marshal(packedToken{
		Issuer:        packedTokenIssuer,
		User:          user,
		Password:      password,
		UpstreamToken: upstreamToken,
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

func getBearerToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return token, true
}

// Returns the upstream token inside, if the request carries a Pavonis token with valid credentials
func checkPackedToken(authMgr *auth.UserManager, r *http.Request) (string, bool) {
	s, ok := getBearerToken(r)
	if !ok {
		return "", false
	}
	token, ok := unpackToken(s)
	if !ok || !authMgr.Check(token.User, token.Password) {
		return "", false
	}
	return token.UpstreamToken, true
}

func setUpstreamBearerToken(r *http.Request, upstreamToken string) {
	if upstreamToken == "" {
		r.Header.Del("Authorization")
	} else {
		r.Header.Set("Authorization", "Bearer "+upstreamToken)
	}
}

func writeTokenResponse(w http.ResponseWriter, token string) {
	data, _ := json.Marshal(map[string]string{"token": token})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func writeChallenge(w http.ResponseWriter, realm, service, scope string) {
	w.Header().Set("Www-Authenticate", buildBearerChallenge(realm, service, scope))
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}

func writeApiVersionCheckOk(w http.ResponseWriter) {
	// https://distribution.github.io/distribution/spec/api/#api-version-check
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}

var challengeParamPattern = regexp.MustCompile(`(\w+)="([^"]*)"`)

func parseBearerChallenge(header string) (map[string]string, bool) {
	scheme, rest, _ := strings.Cut(strings.TrimSpace(header), " ")
	if !strings.EqualFold(scheme, "Bearer") {
		return nil, false
	}
	params := make(map[string]string)
	for _, match := range challengeParamPattern.FindAllStringSubmatch(rest, -1) {
		params[strings.ToLower(match[1])] = match[2]
	}
	return params, true
}

func buildBearerChallenge(realm, service, scope string) string {
	parts := []string{fmt.Sprintf(`realm=%s`, strconv.Quote(realm))}
	if service != "" {
		parts = append(parts, fmt.Sprintf(`service=%s`, strconv.Quote(service)))
	}
	if scope != "" {
		parts = append(parts, fmt.Sprintf(`scope=%s`, strconv.Quote(scope)))
	}
	return "Bearer " + strings.Join(parts, ",")
}

// Sends an unauthenticated "GET /v2/" to the upstream, to learn its token service.
// A nil challenge without error means the upstream does not require auth
func probeUpstreamChallenge(ctx *context.RequestContext, helper *common.RequestHelper, r *http.Request, v2RootUrl *url.URL) (map[string]string, error) {
	status, header, err := helper.FetchHeader(ctx, r, v2RootUrl)
	if err != nil {
		return nil, err
	}
	if status >= 200 && status < 300 {
		return nil, nil
	}
	if status == http.StatusUnauthorized {
		if params, ok := parseBearerChallenge(header.Get("Www-Authenticate")); ok && params["realm"] != "" {
			return params, nil
		}
	}
	return nil, fmt.Errorf("unexpected upstream response to %s: status %d, Www-Authenticate %+q", v2RootUrl, status, header.Get("Www-Authenticate"))
}

func rewriteAccountQuery(r *http.Request, upstreamUser *string) {
	query := r.URL.Query()
	if !query.Has("account") {
		return
	}
	if upstreamUser != nil {
		query.Set("account", *upstreamUser)
	} else {
		query.Del("account")
	}
	r.URL.RawQuery = query.Encode()
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
				fields[key], _ = json.Marshal(packToken(user, password, upstreamToken))
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
