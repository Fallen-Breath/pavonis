package crproxy

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/auth"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
)

type singleProxyHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.ContainerRegistrySingleProxySettings

	selfUrl              *url.URL
	upstreamV1Url        *url.URL // might be nil
	upstreamV2Url        *url.URL
	upstreamAuthRealmUrl *url.URL     // might be nil, use getter-setter to access
	uaruMutex            sync.RWMutex // protects upstreamAuthRealmUrl
	whitelist            *reposList
	blacklist            *reposList
	authMgr              *auth.UserManager
}

var _ handler.HttpHandler = &singleProxyHandler{}

func NewContainerRegistrySingleProxyHandler(info *handler.Info, helper *common.RequestHelper, settings *config.ContainerRegistrySingleProxySettings) (handler.HttpHandler, error) {
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
	if settings.UpstreamAuthRealmUrl != nil {
		if upstreamAuthRealmUrl, err = url.Parse(*settings.UpstreamAuthRealmUrl); err != nil {
			return nil, fmt.Errorf("invalid upstreamAuthRealmUrl %v: %v", settings.UpstreamAuthRealmUrl, err)
		}
	}
	authMgr, err := auth.NewUserManager(info.Id, settings.Auth)
	if err != nil {
		return nil, fmt.Errorf("failed to build auth user list: %v", err)
	}

	h := &singleProxyHandler{
		info:     info,
		helper:   helper,
		settings: settings,

		selfUrl:              selfUrl,
		upstreamV1Url:        upstreamV1Url,
		upstreamV2Url:        upstreamV2Url,
		upstreamAuthRealmUrl: upstreamAuthRealmUrl,
		whitelist:            newReposList(settings.ReposWhitelist),
		blacklist:            newReposList(settings.ReposBlacklist),
		authMgr:              authMgr,
	}

	return h, nil
}

func (h *singleProxyHandler) Info() *handler.Info {
	return h.info
}

func (h *singleProxyHandler) Shutdown() {
	h.authMgr.Shutdown()
}

var realmPattern = regexp.MustCompile(`realm="([^"]+)"`)

var layerUploadLocationPathPattern = regexp.MustCompile(`^/v2/.+/blobs/uploads/[^/]*$`)

func (h *singleProxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	reqPath, ok := utils.CutPathPrefix(r.URL.Path, h.info.PathPrefix)
	if !ok {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, h.info.PathPrefix))
	}

	targetUrl, routePrefix, getRouteOk := h.getRoute(w, reqPath)
	if !getRouteOk {
		return
	}

	// method / path checks
	if !h.checkAllowPush(w, r, routePrefix) {
		return
	}
	if !h.checkAllowList(w, reqPath, routePrefix) {
		return
	}

	// auth check
	cancelled, authResponseModifier := h.handleAuth(ctx, w, r, reqPath, routePrefix)
	if cancelled {
		return
	}

	// whitelist check
	if !h.checkReposWhitelist(ctx, w, reqPath, routePrefix) {
		return
	}

	responseModifier := h.createResponseModifier(ctx, routePrefix)
	var opts []common.ReverseProxyOption
	if routePrefix == routePrefixAuthRealm {
		realmUrl, err := h.getOrProbeUpstreamAuthRealmUrl(ctx, r)
		if err != nil {
			log.Warnf("%sFailed to probe the upstream auth realm: %v", ctx.LogPrefix, err)
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}
		if realmUrl == nil {
			http.Error(w, "The upstream does not require auth", http.StatusNotFound)
			return
		}
		targetUrl = realmUrl
		opts = append(opts, common.WithRequestSizeLimit(maxAuthRequestBodySize))
		if authResponseModifier != nil {
			responseModifier = authResponseModifier
		}
	}
	opts = append(opts, common.WithResponseModifier(responseModifier))

	// actual reverse proxy
	upstreamUrl := *r.URL
	upstreamUrl.Scheme = targetUrl.Scheme
	upstreamUrl.Host = targetUrl.Host
	upstreamUrl.Path = targetUrl.Path + reqPath[len(routePrefix):]
	if routePrefix == routePrefixAuthRealm {
		query := targetUrl.Query()
		for key, values := range r.URL.Query() {
			query[key] = values
		}
		upstreamUrl.RawQuery = query.Encode()
	}

	h.helper.RunReverseProxy(ctx, w, r, &upstreamUrl, opts...)
}

func (h *singleProxyHandler) checkAllowPush(w http.ResponseWriter, r *http.Request, routePrefix routePrefix) bool {
	// "POST /auth" is an OAuth token request, not a push
	if routePrefix == routePrefixAuthRealm {
		return true
	}

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

func (h *singleProxyHandler) checkAllowList(w http.ResponseWriter, reqPath string, routePrefix routePrefix) bool {
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

func (h *singleProxyHandler) rewriteV2ResponseUrlToSelf(u *url.URL) *url.URL {
	if u.Scheme != h.upstreamV2Url.Scheme || u.Host != h.upstreamV2Url.Host {
		return nil
	}
	rest, ok := utils.CutPathPrefix(u.EscapedPath(), h.upstreamV2Url.EscapedPath())
	if !ok {
		return nil
	}

	rewritten := *u
	rewritten.Scheme = h.selfUrl.Scheme
	rewritten.Host = h.selfUrl.Host
	pathPrefix := h.info.PathPrefix
	if h.upstreamV2Url.Path != "" {
		pathPrefix += string(routePrefixV2)
	}
	rewritten.RawPath = (&url.URL{Path: pathPrefix}).EscapedPath() + rest
	rewritten.Path, _ = url.PathUnescape(rewritten.RawPath)
	return &rewritten
}

func (h *singleProxyHandler) createResponseModifier(ctx *context.RequestContext, routePrefix routePrefix) common.ResponseModifier {
	return func(lastReq *http.Request, resp *http.Response) error {
		// https://distribution.github.io/distribution/spec/api/#pagination
		// https://distribution.github.io/distribution/spec/api/#tags-paginated
		common.RewriteLinkHeaderUrls(&resp.Header, func(u *url.URL) *url.URL {
			return h.rewriteV2ResponseUrlToSelf(lastReq.URL.ResolveReference(u))
		}, nil)

		if routePrefix == routePrefixV2 && resp.StatusCode == http.StatusUnauthorized {
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
					h.setUpstreamAuthRealmUrlIfUnset(ctx, oldRealmUrl)
					if oldRealmUrl.String() != h.getUpstreamAuthRealmUrl().String() {
						log.Warnf("%sThe auth realm in the Www-Authenticate does not match the configured value, configured %+q, got %+q", ctx.LogPrefix, h.getUpstreamAuthRealmUrl().String(), oldRealmUrl.String())
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
				rewritten := h.rewriteV2ResponseUrlToSelf(location)
				if rewritten != nil && layerUploadLocationPathPattern.MatchString(strings.TrimPrefix(rewritten.Path, h.info.PathPrefix)) {
					newLocation := rewritten.String()
					log.Debugf("%sRewriting HTTP 202 response Location header from %+q to %+q", ctx.LogPrefix, location.String(), newLocation)
					resp.Header.Set("Location", newLocation)
				} else {
					log.Debugf("%sIgnored unknown HTTP 202 response Location header %+q", ctx.LogPrefix, location.String())
				}
			}
		}
		return nil
	}
}
