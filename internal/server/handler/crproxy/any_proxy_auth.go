package crproxy

import (
	gocontext "context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	log "github.com/sirupsen/logrus"
)

// Returned by any-proxy "/auth" when auth is disabled and the upstream does not require auth,
// since clients reject an empty token
const dummyAuthToken = "pavonis-dummy-token"

func (h *anyProxyHandler) selfAuthRealmUrl() string {
	return h.info.SelfUrl + h.info.PathPrefix + string(routePrefixAuthRealm)
}

func (h *anyProxyHandler) serveApiVersionCheck(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	if h.settings.Auth.Enabled {
		if _, ok := checkPackedToken(h.authMgr, r); ok {
			log.Debugf("%sMocking a successful /v2/ result for a Pavonis token", ctx.LogPrefix)
			writeApiVersionCheckOk(w)
			return
		}
	}
	writeChallenge(w, h.selfAuthRealmUrl(), "", "")
}

func (h *anyProxyHandler) createV2ResponseModifier(registry string, requestHasToken bool) common.ResponseModifier {
	return func(lastReq *http.Request, resp *http.Response) error {
		// https://distribution.github.io/distribution/spec/api/#tags-paginated
		common.RewriteLinkHeaderUrls(&resp.Header, func(u *url.URL) *url.URL {
			return h.rewriteV2ResponseUrlToSelf(registry, lastReq.URL.ResolveReference(u))
		}, nil)

		if resp.StatusCode != http.StatusUnauthorized {
			return nil
		}
		if requestHasToken {
			// The token is already for the right scope, so let the client fail instead of retrying
			resp.Header.Del("Www-Authenticate")
			return nil
		}
		if challenge, ok := parseBearerChallenge(resp.Header.Get("Www-Authenticate")); ok {
			resp.Header.Set("Www-Authenticate", buildBearerChallenge(h.selfAuthRealmUrl(), challenge["service"], prefixScopesWithRegistry(challenge["scope"], registry)))
		}
		return nil
	}
}

func prefixScopesWithRegistry(scopes string, registry string) string {
	var result []string
	for _, scope := range strings.Fields(scopes) {
		if name, ok := strings.CutPrefix(scope, "repository:"); ok {
			scope = "repository:" + registry + "/" + name
		}
		result = append(result, scope)
	}
	return strings.Join(result, " ")
}

// Parses "repository:<registry>/<repo>:<actions>" scopes that share the same registry,
// and returns the registry and the scopes without the registry prefix
func parseAuthScopes(scopes []string) (registry string, stripped []string, ok bool) {
	for _, scope := range scopes {
		rest, isRepository := strings.CutPrefix(scope, "repository:")
		idx := strings.LastIndex(rest, ":")
		if !isRepository || idx == -1 {
			return "", nil, false
		}
		name, actions := rest[:idx], rest[idx+1:]
		scopeRegistry, repo, found := strings.Cut(name, "/")
		if !found || repo == "" || !isRegistryHost(scopeRegistry) || (registry != "" && scopeRegistry != registry) {
			return "", nil, false
		}
		registry = scopeRegistry
		stripped = append(stripped, fmt.Sprintf("repository:%s:%s", repo, actions))
	}
	return registry, stripped, registry != ""
}

func (h *anyProxyHandler) serveAuth(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	var selfUser, selfPassword string
	if h.settings.Auth.Enabled {
		var upstreamUser *string
		var ok bool
		_, _, selfUser, selfPassword, upstreamUser, _, ok = parseBasicAuth(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if upstreamUser != nil {
			http.Error(w, "Upstream credentials are not supported", http.StatusUnauthorized)
			return
		}
		if !h.authMgr.Check(selfUser, selfPassword) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		rewriteAccountQuery(r, nil)
	}
	r.Header.Del("Authorization")

	query := r.URL.Query()
	var scopes []string
	for _, s := range query["scope"] {
		scopes = append(scopes, strings.Fields(s)...)
	}
	if len(scopes) == 0 {
		if !h.settings.Auth.Enabled {
			http.Error(w, "Login is not supported", http.StatusBadRequest)
			return
		}
		log.Debugf("%sIssuing a Pavonis-only token for a login request", ctx.LogPrefix)
		writeTokenResponse(w, packToken(selfUser, selfPassword, ""))
		return
	}
	registry, strippedScopes, ok := parseAuthScopes(scopes)
	if !ok {
		http.Error(w, "Invalid scope", http.StatusBadRequest)
		return
	}

	scheme, challenge, err := probeUpstreamChallenge(ctx, h.helper, r, &url.URL{Scheme: "https", Host: registryUpstreamHost(registry), Path: "/v2/"})
	if err != nil {
		log.Warnf("%sFailed to probe the upstream auth challenge: %v", ctx.LogPrefix, err)
		status := http.StatusBadGateway
		deadline, hasDeadline := r.Context().Deadline()
		if errors.Is(err, gocontext.DeadlineExceeded) || (hasDeadline && !time.Now().Before(deadline)) {
			status = http.StatusGatewayTimeout
		} else if ctx.RequestRateLimited {
			status = http.StatusTooManyRequests
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	switch scheme {
	case upstreamAuthNone:
		if h.settings.Auth.Enabled {
			writeTokenResponse(w, packToken(selfUser, selfPassword, ""))
		} else {
			writeTokenResponse(w, dummyAuthToken)
		}
		return
	case upstreamAuthBasic:
		log.Warnf("%sThe upstream registry %+q uses Basic auth, which requires upstream credentials and is not supported", ctx.LogPrefix, registry)
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
		return
	}
	realmUrl, err := url.Parse(challenge["realm"])
	if err != nil || (realmUrl.Scheme != "https" && realmUrl.Scheme != "http") {
		log.Warnf("%sInvalid upstream auth realm url %+q", ctx.LogPrefix, challenge["realm"])
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}

	query.Del("scope")
	query.Del("service")
	if challenge["service"] != "" {
		query.Set("service", challenge["service"])
	}
	query["scope"] = strippedScopes
	upstreamQuery := realmUrl.Query()
	for key, values := range query {
		upstreamQuery[key] = values
	}
	upstreamUrl := *realmUrl
	upstreamUrl.RawQuery = upstreamQuery.Encode()

	var opts []common.ReverseProxyOption
	if h.settings.Auth.Enabled {
		prepareTokenRequest(r)
		opts = append(opts, common.WithResponseModifier(packTokenResponseModifier(selfUser, selfPassword)))
	}
	h.helper.RunReverseProxy(ctx, w, r, &upstreamUrl, opts...)
}
