package crproxy

import (
	"fmt"
	"net/http"
	"net/url"
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
	settings *config.ContainerRegistryAnyProxySettings

	authMgr   *auth.UserManager
	whitelist *reposList
	blacklist *reposList
}

var _ handler.HttpHandler = &anyProxyHandler{}

var validHostPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-.]*[a-zA-Z0-9])?(:[0-9]{1,5})?$`)

const dockerHubRegistry = "docker.io"
const dockerHubUpstreamHost = "registry-1.docker.io"

func NewContainerRegistryAnyProxyHandler(info *handler.Info, helper *common.RequestHelper, settings *config.ContainerRegistryAnyProxySettings) (handler.HttpHandler, error) {
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
		upstreamToken, ok := checkPackedToken(h.authMgr, r)
		if !ok {
			scope := fmt.Sprintf("repository:%s/%s:pull", registry, strings.Join(reposName, "/"))
			writeChallenge(w, h.selfAuthRealmUrl(), "", scope)
			return
		}
		setUpstreamBearerToken(r, upstreamToken)
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

func (h *anyProxyHandler) createV2ResponseModifier(registry string, requestHasToken bool) common.ResponseModifier {
	return func(_ *http.Request, resp *http.Response) error {
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
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
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
			http.Error(w, "Login is not supported when auth is disabled", http.StatusBadRequest)
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

	challenge, err := probeUpstreamChallenge(ctx, h.helper, r, &url.URL{Scheme: "https", Host: registryUpstreamHost(registry), Path: "/v2/"})
	if err != nil {
		log.Warnf("%sFailed to probe the upstream auth challenge: %v", ctx.LogPrefix, err)
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}
	if challenge == nil {
		// The upstream does not require auth
		if h.settings.Auth.Enabled {
			writeTokenResponse(w, packToken(selfUser, selfPassword, ""))
		} else {
			writeTokenResponse(w, dummyAuthToken)
		}
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

func (h *anyProxyHandler) Info() *handler.Info {
	return h.info
}

func (h *anyProxyHandler) Shutdown() {
	h.authMgr.Shutdown()
}
