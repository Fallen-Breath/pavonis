package crproxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	log "github.com/sirupsen/logrus"
)

// handled is true if the response has been written. A non-nil modifier is applied to the upstream response
func (h *singleProxyHandler) handleAuth(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request, reqPath string, routePrefix routePrefix) (handled bool, modifier common.ResponseModifier) {
	if !h.settings.Auth.Enabled {
		return false, nil
	}

	switch routePrefix {
	case routePrefixV1:
		// V1 search normally sends Basic credentials directly, without going through /auth.
		if _, _, selfUser, selfPassword, upstreamUser, upstreamPassword, ok := parseBasicAuth(r); ok && h.authMgr.Check(selfUser, selfPassword) {
			if upstreamUser != nil && upstreamPassword != nil {
				r.SetBasicAuth(*upstreamUser, *upstreamPassword)
			} else {
				r.Header.Del("Authorization")
			}
		} else if upstreamToken, ok := checkPackedToken(h.authMgr, r); ok {
			setUpstreamBearerToken(r, upstreamToken)
		} else {
			w.Header().Set("WWW-Authenticate", `Basic realm="pavonis"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return true, nil
		}

	case routePrefixAuthRealm:
		if reqPath != string(routePrefixAuthRealm) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return true, nil
		}
		// Rejecting with 401 makes containerd fall back from the OAuth POST to GET with basic auth
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return true, nil
		}

		_, _, selfUser, selfPassword, upstreamUser, upstreamPassword, ok := parseBasicAuth(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return true, nil
		}
		if !h.authMgr.Check(selfUser, selfPassword) {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return true, nil
		}

		if upstreamUser != nil && upstreamPassword != nil {
			r.SetBasicAuth(*upstreamUser, *upstreamPassword)
		} else {
			r.Header.Del("Authorization")
		}
		rewriteAccountQuery(r, upstreamUser)

		if !r.URL.Query().Has("scope") && upstreamUser == nil {
			// A Pavonis-only login request, e.g. from `docker login`.
			// Some upstreams (e.g. ghcr.io) reject token requests without a scope, so do not forward it
			log.Debugf("%sIssuing a Pavonis-only token for a login request", ctx.LogPrefix)
			writeTokenResponse(w, packToken(selfUser, selfPassword, ""))
			return true, nil
		}

		realmUrl, err := h.getOrProbeUpstreamAuthRealmUrl(ctx, r)
		if err != nil {
			log.Warnf("%sFailed to probe the upstream auth realm: %v", ctx.LogPrefix, err)
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return true, nil
		}
		if realmUrl == nil {
			// The upstream does not require auth
			writeTokenResponse(w, packToken(selfUser, selfPassword, ""))
			return true, nil
		}

		prepareTokenRequest(r)
		return false, packTokenResponseModifier(selfUser, selfPassword)

	case routePrefixV2:
		upstreamToken, ok := checkPackedToken(h.authMgr, r)
		if !ok {
			h.writeUnauthorized(ctx, w, r, reqPath)
			return true, nil
		}
		if reqPath == "/v2/" && upstreamToken == "" {
			log.Debugf("%sMocking a successful %s result for a Pavonis-only token", ctx.LogPrefix, reqPath)
			writeApiVersionCheckOk(w)
			return true, nil
		}
		setUpstreamBearerToken(r, upstreamToken)
	}
	return false, nil
}

func (h *singleProxyHandler) writeUnauthorized(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request, reqPath string) {
	challenge, err := h.probeUpstreamChallenge(ctx, r)
	if err != nil {
		log.Warnf("%sFailed to probe the upstream auth challenge: %v", ctx.LogPrefix, err)
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}

	service := ""
	if challenge != nil {
		service = challenge["service"]
	}
	writeChallenge(w, h.selfAuthRealmUrl(), service, challengeScope(reqPath, r.Method))
}

func challengeScope(reqPath string, method string) string {
	reposName, ok := extractReposNameFromV2Path(reqPath)
	if !ok || len(reposName) == 0 {
		return ""
	}
	actions := "pull"
	if method != http.MethodGet && method != http.MethodHead {
		actions = "pull,push"
	}
	return fmt.Sprintf("repository:%s:%s", strings.Join(reposName, "/"), actions)
}

func (h *singleProxyHandler) selfAuthRealmUrl() string {
	return h.info.SelfUrl + h.info.PathPrefix + string(routePrefixAuthRealm)
}

// Also learns the auth realm from the challenge
func (h *singleProxyHandler) probeUpstreamChallenge(ctx *context.RequestContext, r *http.Request) (map[string]string, error) {
	v2RootUrl := *h.upstreamV2Url
	v2RootUrl.Path += "/"
	challenge, err := probeUpstreamChallenge(ctx, h.helper, r, &v2RootUrl)
	if err != nil || challenge == nil {
		return challenge, err
	}
	realmUrl, err := url.Parse(challenge["realm"])
	if err != nil {
		return nil, fmt.Errorf("invalid auth realm url %+q: %v", challenge["realm"], err)
	}
	h.setUpstreamAuthRealmUrlIfUnset(ctx, realmUrl)
	return challenge, nil
}

// A nil result without error means the upstream does not require auth
func (h *singleProxyHandler) getOrProbeUpstreamAuthRealmUrl(ctx *context.RequestContext, r *http.Request) (*url.URL, error) {
	if realmUrl := h.getUpstreamAuthRealmUrl(); realmUrl != nil {
		return realmUrl, nil
	}
	if _, err := h.probeUpstreamChallenge(ctx, r); err != nil {
		return nil, err
	}
	return h.getUpstreamAuthRealmUrl(), nil
}

func (h *singleProxyHandler) getUpstreamAuthRealmUrl() *url.URL {
	h.uaruMutex.RLock()
	defer h.uaruMutex.RUnlock()
	return h.upstreamAuthRealmUrl
}

func (h *singleProxyHandler) setUpstreamAuthRealmUrlIfUnset(ctx *context.RequestContext, url *url.URL) {
	if h.getUpstreamAuthRealmUrl() != nil {
		return
	}
	h.uaruMutex.Lock()
	defer h.uaruMutex.Unlock()

	if h.upstreamAuthRealmUrl != nil {
		return
	}

	log.Printf("%sSetting UpstreamAuthRealmUrl to %+v", ctx.LogPrefix, url)
	h.upstreamAuthRealmUrl = url
}
