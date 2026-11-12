package crproxy

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
)

const maxAuthRequestBodySize = 64 * 1024

func parseBasicAuth(r *http.Request) (username, password, selfUser, selfPassword string, upstreamUser, upstreamPassword *string, ok bool) {
	username, password, ok = r.BasicAuth()
	if !ok {
		return
	}

	splitString := func(s string) (string, *string) {
		parts := strings.SplitN(s, "$", 2)
		if len(parts) != 2 {
			return s, nil
		} else {
			return parts[0], &parts[1]
		}
	}

	selfUser, upstreamUser = splitString(username)
	selfPassword, upstreamPassword = splitString(password)
	return
}

func getBearerToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return token, true
}

func bearerAuthorization(token string) string {
	if token == "" {
		return ""
	}
	return "Bearer " + token
}

func basicAuthorization(user, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+password))
}

func setUpstreamAuthorization(r *http.Request, upstreamAuthorization string) {
	if upstreamAuthorization == "" {
		r.Header.Del("Authorization")
	} else {
		r.Header.Set("Authorization", upstreamAuthorization)
	}
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

func isBasicChallenge(header string) bool {
	scheme, _, _ := strings.Cut(strings.TrimSpace(header), " ")
	return strings.EqualFold(scheme, "Basic")
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

type upstreamAuthScheme int

const (
	upstreamAuthNone upstreamAuthScheme = iota
	upstreamAuthBearer
	upstreamAuthBasic
)

// Sends an unauthenticated "GET /v2/" to the upstream, to learn how it authenticates.
// The Bearer challenge params are returned for upstreamAuthBearer only
func probeUpstreamChallenge(ctx *context.RequestContext, helper *common.RequestHelper, r *http.Request, v2RootUrl *url.URL) (upstreamAuthScheme, map[string]string, error) {
	status, header, err := helper.FetchHeader(ctx, r, v2RootUrl)
	if err != nil {
		return 0, nil, err
	}
	if status >= 200 && status < 300 {
		return upstreamAuthNone, nil, nil
	}
	if status == http.StatusUnauthorized {
		authenticate := header.Get("Www-Authenticate")
		if params, ok := parseBearerChallenge(authenticate); ok && params["realm"] != "" {
			return upstreamAuthBearer, params, nil
		}
		if isBasicChallenge(authenticate) {
			return upstreamAuthBasic, nil, nil
		}
	}
	return 0, nil, fmt.Errorf("unexpected upstream response to %s: status %d, Www-Authenticate %+q", v2RootUrl, status, header.Get("Www-Authenticate"))
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
