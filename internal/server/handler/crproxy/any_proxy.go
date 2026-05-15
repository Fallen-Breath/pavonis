package crproxy

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	log "github.com/sirupsen/logrus"
)

type anyProxyHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.ContainerRegistryAnyProxySettings

	selfUrl    *url.URL
	authMgr    *authManager
	whitelist  *reposList
	blacklist  *reposList
	realmCache sync.Map // map[string]*url.URL
}

var _ handler.HttpHandler = &anyProxyHandler{}

var validHostPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-.]*[a-zA-Z0-9])?(:[0-9]{1,5})?$`)

func NewContainerRegistryAnyProxyHandler(info *handler.Info, helper *common.RequestHelper, settings *config.ContainerRegistryAnyProxySettings) (handler.HttpHandler, error) {
	selfUrl, err := url.Parse(info.SelfUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid SelfUrl %v: %v", info.SelfUrl, err)
	}
	authMgr, err := newAuthManager(info.Id, settings.Auth)
	if err != nil {
		return nil, fmt.Errorf("failed to build auth user list: %v", err)
	}
	h := &anyProxyHandler{
		info:      info,
		helper:    helper,
		settings:  settings,
		selfUrl:   selfUrl,
		authMgr:   authMgr,
		whitelist: newReposList(settings.ReposWhitelist),
		blacklist: newReposList(settings.ReposBlacklist),
	}
	h.authMgr.StartBackgroundReload()
	return h, nil
}

func (h *anyProxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, h.info.PathPrefix) {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, h.info.PathPrefix))
	}
	reqPath := r.URL.Path[len(h.info.PathPrefix):]

	registryHost, routedPath, ok := splitAnyProxyPath(reqPath)
	if !ok {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if !validHostPattern.MatchString(registryHost) {
		http.Error(w, "Invalid registry host", http.StatusBadRequest)
		return
	}

	var targetUrl *url.URL
	var route routePrefix
	if routedPath == string(routePrefixAuthRealm) {
		realmUrl, loaded := h.loadRealm(registryHost)
		if !loaded || realmUrl == nil {
			http.Error(w, "The auth-realm URL is not available yet", http.StatusServiceUnavailable)
			return
		}
		targetUrl = realmUrl
		route = routePrefixAuthRealm
	} else if strings.HasPrefix(routedPath, string(routePrefixV2)) {
		targetUrl = &url.URL{Scheme: "https", Host: registryHost}
		route = routePrefixV2
	} else {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !h.checkAllowList(w, routedPath, route) {
		return
	}
	if h.handleAuth(ctx, w, r, routedPath, route) {
		return
	}
	if !h.checkReposWhitelist(ctx, w, routedPath, route, registryHost) {
		return
	}

	downstreamUrl := *r.URL
	downstreamUrl.Scheme = targetUrl.Scheme
	downstreamUrl.Host = targetUrl.Host
	if route == routePrefixAuthRealm {
		downstreamUrl.Path = targetUrl.Path
		downstreamUrl.RawQuery = r.URL.RawQuery
		if targetUrl.RawQuery != "" {
			if downstreamUrl.RawQuery == "" {
				downstreamUrl.RawQuery = targetUrl.RawQuery
			} else {
				downstreamUrl.RawQuery = targetUrl.RawQuery + "&" + downstreamUrl.RawQuery
			}
		}
	} else {
		downstreamUrl.Path = routedPath
	}

	responseModifier := h.createResponseModifier(ctx, registryHost, route)
	h.helper.RunReverseProxy(ctx, w, r, &downstreamUrl, common.WithResponseModifier(responseModifier))
}

func splitAnyProxyPath(path string) (registryHost string, routedPath string, ok bool) {
	if !strings.HasPrefix(path, "/") {
		return "", "", false
	}
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return "", "", false
	}
	idx := strings.Index(trimmed, "/")
	if idx == -1 {
		return trimmed, "/", true
	}
	registryHost = trimmed[:idx]
	routedPath = trimmed[idx:]
	return registryHost, routedPath, registryHost != ""
}

func (h *anyProxyHandler) checkAllowList(w http.ResponseWriter, reqPath string, route routePrefix) bool {
	if *h.settings.AllowList {
		return true
	}
	if route == routePrefixV2 && (strings.HasSuffix(reqPath, "/v2/_catalog") || strings.HasSuffix(reqPath, "/tags/list")) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return false
	}
	return true
}

func (h *anyProxyHandler) checkReposWhitelist(ctx *context.RequestContext, w http.ResponseWriter, reqPath string, route routePrefix, registryHost string) bool {
	if len(*h.whitelist) == 0 && len(*h.blacklist) == 0 {
		return true
	}
	if route != routePrefixV2 {
		return true
	}
	reposName := extractReposNameFromV2Path(reqPath)
	log.Debugf("%sExtracted reposName from reqPath %+q: %+v", ctx.LogPrefix, reqPath, reposName)
	if reposName == nil {
		return true
	}
	withHost := make([]string, 0, len(*reposName)+1)
	withHost = append(withHost, registryHost)
	withHost = append(withHost, (*reposName)...)
	return checkAndApplyWhitelists(h.whitelist, h.blacklist, w, withHost)
}

func (h *anyProxyHandler) handleAuth(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request, reqPath string, route routePrefix) bool {
	if !h.settings.Auth.Enabled {
		return false
	}

	if route == routePrefixAuthRealm && reqPath == string(routePrefixAuthRealm) {
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
			_, _ = w.Write([]byte(fmt.Sprintf(`{"token": "%s"}`, dummyAuthToken)))
			log.Debugf("%sMocking a successful %s result for a Pavonis-only login request", ctx.LogPrefix, reqPath)
			return true
		}
	}
	if route == routePrefixV2 && reqPath == "/v2/" {
		if r.Header.Get("Authorization") == fmt.Sprintf("Bearer %s", dummyAuthToken) {
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
			log.Debugf("%sMocking a successful %s result for a Pavonis-only login request", ctx.LogPrefix, reqPath)
			return true
		}
	}
	return false
}

func (h *anyProxyHandler) loadRealm(registryHost string) (*url.URL, bool) {
	v, ok := h.realmCache.Load(registryHost)
	if !ok {
		return nil, false
	}
	realmUrl, ok := v.(*url.URL)
	if !ok {
		return nil, false
	}
	return realmUrl, true
}

func (h *anyProxyHandler) createResponseModifier(ctx *context.RequestContext, registryHost string, route routePrefix) common.ResponseModifier {
	return func(_ *http.Request, resp *http.Response) error {
		common.RewriteLinkHeaderUrls(&resp.Header, func(u *url.URL) *url.URL {
			if u.Host == registryHost {
				u.Scheme = h.selfUrl.Scheme
				u.Host = h.selfUrl.Host
				u.Path = h.info.PathPrefix + "/" + registryHost + u.Path
				return u
			}
			return nil
		}, nil)

		if route == routePrefixV2 && resp.StatusCode == http.StatusUnauthorized {
			if authHeaders, ok := resp.Header["Www-Authenticate"]; ok && len(authHeaders) > 0 {
				newHeader := realmPattern.ReplaceAllStringFunc(authHeaders[0], func(match string) string {
					submatches := realmPattern.FindStringSubmatch(match)
					if len(submatches) < 2 {
						return match
					}

					oldRealm := submatches[1]
					oldRealmUrl, err := url.Parse(oldRealm)
					if err != nil || oldRealmUrl == nil {
						log.Warnf("%sInvalid auth realm url %+q", ctx.LogPrefix, oldRealm)
						return match
					}

					if old, loaded := h.loadRealm(registryHost); loaded && old.String() != oldRealmUrl.String() {
						log.Warnf("%sThe auth realm in the Www-Authenticate does not match the cached value, cached %+q, got %+q", ctx.LogPrefix, old.String(), oldRealmUrl.String())
					} else {
						h.realmCache.Store(registryHost, oldRealmUrl)
					}

					newRealm := h.info.SelfUrl + h.info.PathPrefix + "/" + registryHost + string(routePrefixAuthRealm)
					return fmt.Sprintf(`realm="%s"`, newRealm)
				})
				resp.Header.Set("Www-Authenticate", newHeader)
			}
		}
		return nil
	}
}

func (h *anyProxyHandler) Info() *handler.Info {
	return h.info
}

func (h *anyProxyHandler) Shutdown() {
	h.authMgr.Shutdown()
}
