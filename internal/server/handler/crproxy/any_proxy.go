package crproxy

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/auth"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
)

type anyProxyHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.ContainerRegistryAnySiteSettings

	authMgr   *auth.UserManager
	whitelist *reposList
	blacklist *reposList
}

var _ handler.HttpHandler = &anyProxyHandler{}

var validHostPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-.]*[a-zA-Z0-9])?(:[0-9]{1,5})?$`)

const dockerHubRegistry = "docker.io"

const dockerHubUpstreamHost = "registry-1.docker.io"

func NewContainerRegistryAnyHandler(info *handler.Info, helper *common.RequestHelper, settings *config.ContainerRegistryAnySiteSettings) (handler.HttpHandler, error) {
	authMgr, err := auth.NewUserManager(info.Id, settings.Auth)
	if err != nil {
		return nil, fmt.Errorf("failed to build auth user list: %v", err)
	}
	h := &anyProxyHandler{
		info:      info,
		helper:    helper,
		settings:  settings,
		authMgr:   authMgr,
		whitelist: newRegistryReposList(settings.ReposWhitelist),
		blacklist: newRegistryReposList(settings.ReposBlacklist),
	}
	return h, nil
}

// Follows the docker rule to tell whether the first segment of an image name is a registry host
func isRegistryHost(s string) bool {
	return (strings.ContainsAny(s, ".:") || s == "localhost") && validHostPattern.MatchString(s)
}

func registryUpstreamHost(registry string) string {
	if registry == dockerHubRegistry {
		return dockerHubUpstreamHost
	}
	return registry
}

func normalizeRegistry(registry string) string {
	if registry == dockerHubUpstreamHost {
		return dockerHubRegistry
	}
	return registry
}

func newRegistryReposList(list []string) *reposList {
	l := newReposList(list)
	for _, ent := range *l {
		ent[0] = normalizeRegistry(ent[0])
	}
	return l
}

func (h *anyProxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	reqPath, ok := utils.CutPathPrefix(r.URL.Path, h.info.PathPrefix)
	if !ok {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, h.info.PathPrefix))
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	switch {
	case reqPath == string(routePrefixAuthRealm):
		h.serveAuth(ctx, w, r)
	case reqPath == "/v2/":
		h.serveApiVersionCheck(ctx, w, r)
	case strings.HasPrefix(reqPath, "/v2/"):
		h.serveV2(ctx, w, r, reqPath)
	default:
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}

// "/v2/<registry>/<repo>/..." -> registry, "/v2/<repo>/..."
func splitAnyProxyV2Path(reqPath string) (registry string, upstreamPath string, ok bool) {
	rest := strings.TrimPrefix(reqPath, "/v2/")
	registry, rest, ok = strings.Cut(rest, "/")
	if !ok || rest == "" {
		return "", "", false
	}
	return registry, "/v2/" + rest, true
}

func (h *anyProxyHandler) serveV2(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request, reqPath string) {
	if strings.HasSuffix(reqPath, "/_catalog") || strings.HasSuffix(reqPath, "/tags/list") {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	registry, upstreamPath, ok := splitAnyProxyV2Path(reqPath)
	if !ok {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if !isRegistryHost(registry) {
		http.Error(w, "Invalid registry host", http.StatusBadRequest)
		return
	}
	reposName, ok := extractReposNameFromV2Path(upstreamPath)
	if !ok || len(reposName) == 0 {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	_, hasToken := getBearerToken(r)
	if h.settings.Auth.Enabled {
		upstreamAuthorization, ok := checkPackedToken(h.authMgr, r)
		if !ok {
			scope := fmt.Sprintf("repository:%s/%s:pull", registry, strings.Join(reposName, "/"))
			writeChallenge(w, h.selfAuthRealmUrl(), "", scope)
			return
		}
		setUpstreamAuthorization(r, upstreamAuthorization)
	} else if token, _ := getBearerToken(r); token == dummyAuthToken {
		r.Header.Del("Authorization")
	}

	if !h.checkReposWhitelist(ctx, w, registry, reposName) {
		return
	}

	upstreamUrl := *r.URL
	upstreamUrl.Scheme = "https"
	upstreamUrl.Host = registryUpstreamHost(registry)
	upstreamUrl.Path = upstreamPath
	upstreamUrl.RawPath = ""

	responseModifier := h.createV2ResponseModifier(registry, hasToken)
	h.helper.RunReverseProxy(ctx, w, r, &upstreamUrl, common.WithResponseModifier(responseModifier))
}

func (h *anyProxyHandler) checkReposWhitelist(ctx *context.RequestContext, w http.ResponseWriter, registry string, reposName []string) bool {
	if len(*h.whitelist) == 0 && len(*h.blacklist) == 0 {
		return true
	}
	withRegistry := append([]string{normalizeRegistry(registry)}, reposName...)
	log.Debugf("%sChecking repos lists with %+v", ctx.LogPrefix, withRegistry)
	return checkAndApplyWhitelists(h.whitelist, h.blacklist, w, withRegistry)
}

func (h *anyProxyHandler) Info() *handler.Info {
	return h.info
}

func (h *anyProxyHandler) Shutdown() {
	h.authMgr.Shutdown()
}
