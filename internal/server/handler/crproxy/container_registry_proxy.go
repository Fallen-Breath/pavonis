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
	"time"
)

type proxyHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.ContainerRegistrySettings

	selfUrl          *url.URL
	upstreamV2Url    *url.URL
	upstreamTokenUrl *url.URL
	whitelist        *reposList
	blacklist        *reposList
	authUsers        atomic.Value // type: authUserList
	shutdownChannel  chan bool
}

var _ handler.HttpHandler = &proxyHandler{}

func NewContainerRegistryProxyHandler(info *handler.Info, helper *common.RequestHelper, settings *config.ContainerRegistrySettings) (handler.HttpHandler, error) {
	var err error
	var selfUrl, upstreamV2Url, upstreamTokenUrl *url.URL
	if selfUrl, err = url.Parse(info.SelfUrl); err != nil {
		return nil, fmt.Errorf("invalid SelfUrl %v: %v", info.SelfUrl, err)
	}
	if upstreamV2Url, err = url.Parse(*settings.UpstreamV2Url); err != nil {
		return nil, fmt.Errorf("invalid UpstreamV2Url %v: %v", settings.UpstreamV2Url, err)
	}
	if upstreamTokenUrl, err = url.Parse(*settings.UpstreamAuthRealmUrl); err != nil {
		return nil, fmt.Errorf("invalid upstreamTokenUrl %v: %v", settings.UpstreamAuthRealmUrl, err)
	}

	h := &proxyHandler{
		info:     info,
		helper:   helper,
		settings: settings,

		selfUrl:          selfUrl,
		upstreamV2Url:    upstreamV2Url,
		upstreamTokenUrl: upstreamTokenUrl,
		whitelist:        newReposList(settings.ReposWhitelist),
		blacklist:        newReposList(settings.ReposBlacklist),
		shutdownChannel:  make(chan bool, 1),
	}

	authUsers, err := buildAuthUserList(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to build auth user list: %v", err)
	}
	h.authUsers.Store(authUsers)

	go h.reloadThread()

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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}

	var targetUrl *url.URL
	var pathPrefix string
	if strings.HasPrefix(reqPath, "/v2") {
		targetUrl = h.upstreamV2Url
		pathPrefix = "/v2"
	} else if strings.HasPrefix(reqPath, "/auth") {
		targetUrl = h.upstreamTokenUrl
		pathPrefix = "/auth"
	} else {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// Authorization hijack
	// NOTES: if Auth is Enabled, upstream authorization will not work,
	// This usually means AllowPush should set to false (otherwise it will be meaningless)
	if h.settings.Auth.Enabled && reqPath == "/auth" {
		selfUser, selfPassword, upstreamUser, upstreamPassword, ok := parseBasicAuth(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if !h.checkForAuthorization(selfUser, selfPassword) {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		if upstreamUser != nil && upstreamPassword != nil {
			r.SetBasicAuth(*upstreamUser, *upstreamPassword)
		} else {
			r.Header.Del("Authorization")
		}
	}

	// whitelist && blacklist check
	if pathPrefix == "/v2" && (len(*h.whitelist) > 0 || len(*h.blacklist) > 0) {
		reposName := extractReposNameFromV2Path(reqPath)
		log.Debugf("%sExtracted reposName from reqPath %+q: %+v", ctx.LogPrefix, reqPath, reposName)
		if reposName != nil && !h.checkAndApplyWhitelists(w, *reposName) {
			return
		}
	}

	downstreamUrl := *r.URL
	downstreamUrl.Scheme = targetUrl.Scheme
	downstreamUrl.Host = targetUrl.Host
	downstreamUrl.Path = targetUrl.Path + reqPath[len(pathPrefix):]

	responseModifier := func(resp *http.Response) error {
		if pathPrefix == "/v2" && resp.StatusCode == http.StatusUnauthorized {
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
					newRealm := h.info.SelfUrl + h.info.PathPrefix + "/auth"

					return fmt.Sprintf(`realm="%s"`, newRealm)
				})
				resp.Header.Set("Www-Authenticate", newHeader)
			}
		}
		if pathPrefix == "/v2" && resp.StatusCode == http.StatusAccepted /* 202 */ {
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

	h.helper.RunReverseProxy(ctx, w, r, &downstreamUrl, common.WithResponseModifier(responseModifier))
}

func (h *proxyHandler) checkForAuthorization(username string, password string) bool {
	authUsers := h.authUsers.Load().(authUserList)
	for _, user := range authUsers {
		if user.Name == username && user.Password == password {
			return true
		}
	}
	return false
}

func (h *proxyHandler) checkAndApplyWhitelists(w http.ResponseWriter, reposName []string) bool {
	if len(*h.whitelist) > 0 && !h.whitelist.Check(reposName) {
		http.Error(w, fmt.Sprintf("Repository '%s' is not whitelisted", strings.Join(reposName, "/")), http.StatusForbidden)
		return false
	}
	if len(*h.blacklist) > 0 && h.blacklist.Check(reposName) {
		http.Error(w, fmt.Sprintf("Repository '%s' is blacklisted", strings.Join(reposName, "/")), http.StatusForbidden)
		return false
	}
	return true
}

func (h *proxyHandler) reloadThread() {
	interval := h.settings.Auth.UsersFileReloadInterval

	needsReloadThread := false
	needsReloadThread = needsReloadThread || (h.settings.Auth.Enabled && interval != nil)
	if !needsReloadThread {
		return
	}

	reloadAuthUserList := func() {
		newAuthUserList, err := buildAuthUserList(h.settings)
		if err != nil {
			log.Errorf("Failed to build auth user list: %v", err)
			return
		}

		h.authUsers.Store(newAuthUserList)
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			reloadAuthUserList()

		case <-h.shutdownChannel:
			break
		}
	}
}
