package crproxy

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/server/context"
	log "github.com/sirupsen/logrus"
)

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

const dummyAuthToken = "pavonis-dummy-token"

// true: cancel the reverse proxy action; false: keep going
func (h *singleProxyHandler) handleAuth(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request, reqPath string, routePrefix routePrefix) bool {
	if !h.settings.Auth.Enabled {
		return false
	}

	if routePrefix == routePrefixAuthRealm && reqPath == string(routePrefixAuthRealm) {
		_, _, selfUser, selfPassword, upstreamUser, upstreamPassword, ok := parseBasicAuth(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return true
		}

		if !h.authMgr.CheckForAuthorization(selfUser, selfPassword) {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return true
		}

		if upstreamUser != nil && upstreamPassword != nil {
			r.SetBasicAuth(*upstreamUser, *upstreamPassword)
		} else {
			r.Header.Del("Authorization")
		}

		if !r.URL.Query().Has("scope") && upstreamUser == nil {
			// The client is requesting the "/auth" endpoint without upstream auth info
			// which might be from an `docker login` CLI command
			// Meanwhile, the upstream registry might reject the auth request (e.g. ghcr.io),
			// since the client request from `docker login` does not contain a "scope=repository:<user>/<image>:pull" query.
			// As a workaround, just let this request pass
			_, _ = w.Write([]byte(fmt.Sprintf(`{"token": "%s"}`, dummyAuthToken)))
			log.Debugf("%sMocking a successful %s result for a Pavonis-only login request", ctx.LogPrefix, reqPath)
			return true
		}

		// XXX: rewrite the "account" query param?
	}
	if routePrefix == routePrefixV2 && reqPath == "/v2/" {
		// Mocking the post-`docker login` request to the "/v2/" endpoint
		if r.Header.Get("Authorization") == fmt.Sprintf("Bearer %s", dummyAuthToken) {
			// https://distribution.github.io/distribution/spec/api/#api-version-check
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
			log.Debugf("%sMocking a successful %s result for a Pavonis-only login request", ctx.LogPrefix, reqPath)
			return true
		}
	}
	return false
}
