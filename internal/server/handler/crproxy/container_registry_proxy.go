package crproxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
)

type proxyHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.ContainerRegistrySettings

	selfUrl              *url.URL
	upstreamV1Url        *url.URL // might be nil
	upstreamV2Url        *url.URL
	upstreamAuthRealmUrl *url.URL
	whitelist            *reposList
	blacklist            *reposList
	authUsers            atomic.Value // type: authUserList
	shutdownChannel      chan bool
}

var _ handler.HttpHandler = &proxyHandler{}

func NewContainerRegistryProxyHandler(info *handler.Info, helper *common.RequestHelper, settings *config.ContainerRegistrySettings) (handler.HttpHandler, error) {
	var err error
	var selfUrl, upstreamV1Url, upstreamV2Url, upstreamAuthRealmUrl *url.URL
	if selfUrl, err = url.Parse(info.SelfUrl); err != nil {
		return nil, fmt.Errorf("invalid SelfUrl %v: %v", info.SelfUrl, err)
	}
	if settings.UpstreamV1Url != nil {
		if upstreamV1Url, err = url.Parse(*settings.UpstreamV1Url); err != nil {
			return nil, fmt.Errorf("invalid UpstreamV1Url %v: %v", settings.UpstreamV1Url, err)
		}
	}
	if upstreamV2Url, err = url.Parse(*settings.UpstreamV2Url); err != nil {
		return nil, fmt.Errorf("invalid UpstreamV2Url %v: %v", settings.UpstreamV2Url, err)
	}
	if upstreamAuthRealmUrl, err = url.Parse(*settings.UpstreamAuthRealmUrl); err != nil {
		return nil, fmt.Errorf("invalid upstreamAuthRealmUrl %v: %v", settings.UpstreamAuthRealmUrl, err)
	}

	h := &proxyHandler{
		info:     info,
		helper:   helper,
		settings: settings,

		selfUrl:              selfUrl,
		upstreamV1Url:        upstreamV1Url,
		upstreamV2Url:        upstreamV2Url,
		upstreamAuthRealmUrl: upstreamAuthRealmUrl,
		whitelist:            newReposList(settings.ReposWhitelist),
		blacklist:            newReposList(settings.ReposBlacklist),
		shutdownChannel:      make(chan bool, 1),
	}

	authUsers, err := buildAuthUserList(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to build auth user list: %v", err)
	}
	h.authUsers.Store(authUsers)

	go h.backgroundReloadThread()

	return h, nil
}

func (h *proxyHandler) Info() *handler.Info {
	return h.info
}

func (h *proxyHandler) Shutdown() {
	h.shutdownChannel <- true
}

var realmPattern = regexp.MustCompile(`realm="([^"]+)"`)
var layerUploadLocationPathPattern = regexp.MustCompile(`^/v2/.+/blobs/uploads/[^/]*$`)

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, h.info.PathPrefix) {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, h.info.PathPrefix))
	}
	reqPath := r.URL.Path[len(h.info.PathPrefix):]

	targetUrl, routePrefix, getRouteOk := h.getRoute(w, reqPath)
	if !getRouteOk {
		return
	}

	if !h.checkAllowPush(w, r) {
		return
	}
	if !h.checkAllowList(w, reqPath, routePrefix) {
		return
	}
	if !h.handleAuth(w, r, reqPath) {
		return
	}
	if !h.checkReposWhitelist(ctx, w, reqPath, routePrefix) {
		return
	}

	downstreamUrl := *r.URL
	downstreamUrl.Scheme = targetUrl.Scheme
	downstreamUrl.Host = targetUrl.Host
	downstreamUrl.Path = targetUrl.Path + reqPath[len(routePrefix):]

	responseModifier := h.createResponseModifier(ctx, routePrefix)
	h.helper.RunReverseProxy(ctx, w, r, &downstreamUrl, common.WithResponseModifier(responseModifier))
}

func (h *proxyHandler) checkAllowPush(w http.ResponseWriter, r *http.Request) bool {
	// https://distribution.github.io/distribution/spec/api/#detail
	// GET      /v2/<name>/tags/list
	// GET      /v2/<name>/manifests/<reference>
	// PUT      /v2/<name>/manifests/<reference>
	// DELETE   /v2/<name>/manifests/<reference>
	// GET      /v2/<name>/blobs/<digest>
	// DELETE   /v2/<name>/blobs/<digest>
	// POST     /v2/<name>/blobs/uploads/
	// GET      /v2/<name>/blobs/uploads/<uuid>
	// PATCH    /v2/<name>/blobs/uploads/<uuid>
	// PUT      /v2/<name>/blobs/uploads/<uuid>
	// DELETE   /v2/<name>/blobs/uploads/<uuid>
	// GET      /v2/_catalog
	if !*h.settings.AllowPush {
		// the easiest way to disable push
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return false
		}
	}
	return true
}

func (h *proxyHandler) checkAllowList(w http.ResponseWriter, reqPath string, routePrefix routePrefix) bool {
	if *h.settings.AllowList {
		return true
	}

	forbiddenForListing := false
	if routePrefix == routePrefixV1 {
		forbiddenForListing = true
	} else if routePrefix == routePrefixV2 {
		forbiddenForListing = strings.HasSuffix(reqPath, "/v2/_catalog") || strings.HasSuffix(reqPath, "/tags/list")
	}
	if forbiddenForListing {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return false
	}
	return true
}

func (h *proxyHandler) createResponseModifier(ctx *context.RequestContext, routePrefix routePrefix) func(resp *http.Response) error {
	return func(resp *http.Response) error {
		// https://distribution.github.io/distribution/spec/api/#pagination
		// https://distribution.github.io/distribution/spec/api/#tags-paginated
		common.RewriteLinkHeaderUrls(&resp.Header, func(u *url.URL) *url.URL {
			if u.Scheme == h.upstreamV2Url.Scheme && u.Host == h.upstreamV2Url.Host {
				u.Scheme = h.selfUrl.Scheme
				u.Host = h.selfUrl.Host
				u.Path = h.info.PathPrefix + u.Path
				return u
			}
			return nil
		}, nil)

		if routePrefix == routePrefixV2 && resp.StatusCode == http.StatusUnauthorized {
			if authHeaders, ok := resp.Header["Www-Authenticate"]; ok && len(authHeaders) > 0 {
				newHeader := realmPattern.ReplaceAllStringFunc(authHeaders[0], func(match string) string {
					submatches := realmPattern.FindStringSubmatch(match)
					if len(submatches) < 2 {
						return match
					}

					oldRealm := submatches[1]
					if oldRealm != *h.settings.UpstreamAuthRealmUrl {
						log.Warnf("%sThe auth realm in the Www-Authenticate does not match the configured value, configured %+q, got %+q", ctx.LogPrefix, *h.settings.UpstreamAuthRealmUrl, oldRealm)
					}
					newRealm := h.info.SelfUrl + h.info.PathPrefix + string(routePrefixAuthRealm)

					return fmt.Sprintf(`realm="%s"`, newRealm)
				})
				resp.Header.Set("Www-Authenticate", newHeader)
			}
		}
		if routePrefix == routePrefixV2 && resp.StatusCode == http.StatusAccepted /* 202 */ {
			// Reference: https://distribution.github.io/distribution/spec/api (Search keyword "202 Accepted")
			// "/v2/<name>/blobs/uploads/
			// "/v2/<name>/blobs/uploads/<uuid>"
			if location, err := resp.Location(); err == nil && location != nil {
				urlOk := location.Scheme == h.upstreamV2Url.Scheme && location.Host == h.upstreamV2Url.Host
				pathOk := layerUploadLocationPathPattern.MatchString(location.Path)
				if urlOk && pathOk {
					location.Scheme = h.selfUrl.Scheme
					location.Host = h.selfUrl.Host
					location.Path = h.info.PathPrefix + location.Path
					newLocation := location.String()
					log.Debugf("%sRewriting HTTP 202 response Location header from %+q to %+q", ctx.LogPrefix, location.String(), newLocation)
					resp.Header.Set("Location", newLocation)
				} else {
					log.Debugf("%sIgnored unknown HTTP 202 response Location header %+q (urlOk %v, pathOk %v)", ctx.LogPrefix, location.String(), urlOk, pathOk)
				}
			}
		}
		return nil
	}
}
