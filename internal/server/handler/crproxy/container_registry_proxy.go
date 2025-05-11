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
)

type proxyHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.ContainerRegistrySettings

	upstreamV2Url    *url.URL
	upstreamTokenUrl *url.URL
	whitelist        *reposList
	blacklist        *reposList
}

var _ handler.HttpHandler = &proxyHandler{}

var realmPattern = regexp.MustCompile(`realm="[^"]+"`)

func NewContainerRegistryHandler(info *handler.Info, helper *common.RequestHelper, settings *config.ContainerRegistrySettings) (handler.HttpHandler, error) {
	var err error
	var upstreamV2Url, upstreamTokenUrl *url.URL
	if upstreamV2Url, err = url.Parse(*settings.UpstreamV2Url); err != nil {
		return nil, fmt.Errorf("invalid UpstreamV2Url %v: %v", settings.UpstreamV2Url, err)
	}
	if upstreamTokenUrl, err = url.Parse(*settings.UpstreamTokenUrl); err != nil {
		return nil, fmt.Errorf("invalid upstreamTokenUrl %v: %v", settings.UpstreamTokenUrl, err)
	}

	return &proxyHandler{
		info:     info,
		helper:   helper,
		settings: settings,

		upstreamV2Url:    upstreamV2Url,
		upstreamTokenUrl: upstreamTokenUrl,
		whitelist:        newReposList(settings.ReposWhitelist),
		blacklist:        newReposList(settings.ReposBlacklist),
	}, nil
}

func (h *proxyHandler) Info() *handler.Info {
	return h.info
}

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

	var targetURL *url.URL
	var pathPrefix string
	if strings.HasPrefix(reqPath, "/v2") {
		targetURL = h.upstreamV2Url
		pathPrefix = "/v2"
	} else if strings.HasPrefix(reqPath, "/token") {
		targetURL = h.upstreamTokenUrl
		pathPrefix = "/token"
	} else {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Authorization hijack
	// NOTES: if Authorization is Enabled, upstream authorization will not work,
	// This usually means AllowPush should set to false (otherwise it will be meaningless)
	if h.settings.Authorization.Enabled && reqPath == "/token" {
		username, password, ok := r.BasicAuth()
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !h.checkForAuthorization(username, password) {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Remove the Authorization field, so we will still send an anonymous request to upstream
		r.Header.Del("Authorization")
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
	downstreamUrl.Scheme = targetURL.Scheme
	downstreamUrl.Host = targetURL.Host
	downstreamUrl.Path = targetURL.Path + reqPath[len(pathPrefix):]

	responseModifier := func(resp *http.Response) error {
		if pathPrefix == "/v2" && resp.StatusCode == http.StatusUnauthorized {
			if auth, ok := resp.Header["Www-Authenticate"]; ok && len(auth) > 0 {
				newRealm := h.settings.SelfUrl + h.info.PathPrefix + "/token"
				newAuth := realmPattern.ReplaceAllString(auth[0], `realm="`+newRealm+`"`)
				resp.Header.Set("Www-Authenticate", newAuth)
			}
		}
		return nil
	}

	h.helper.RunReverseProxy(ctx, w, r, &downstreamUrl, responseModifier)
}

func (h *proxyHandler) checkForAuthorization(username string, password string) bool {
	for _, user := range h.settings.Authorization.Users {
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
