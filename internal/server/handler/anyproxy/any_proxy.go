package anyproxy

import (
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/auth"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
)

// Authorization is always consumed by Pavonis, so the upstream's credentials and challenges go through these headers
const (
	headerUpstreamAuthorization   = "X-Pavonis-Upstream-Authorization"
	headerUpstreamWwwAuthenticate = "X-Pavonis-Upstream-WWW-Authenticate"
)

type proxyHandler struct {
	info      *handler.Info
	helper    *common.RequestHelper
	settings  *config.AnyProxySettings
	authMgr   *auth.UserManager
	whitelist domainList
	blacklist domainList
}

var _ handler.HttpHandler = (*proxyHandler)(nil)

func NewHandler(info *handler.Info, helper *common.RequestHelper, settings *config.AnyProxySettings) (handler.HttpHandler, error) {
	authMgr, err := auth.NewUserManager(info.Id, settings.Auth)
	if err != nil {
		return nil, fmt.Errorf("failed to build auth user list: %v", err)
	}
	if !settings.AllowPrivateNetwork {
		helper = helper.WithCheckDirectDialAddr(checkPublicAddr)
	}
	if !isAuthEnabled(settings) && len(settings.DomainWhitelist) == 0 {
		log.Warnf("(%s) Neither auth nor domain whitelist is configured, anyone can use this site to access any website", info.Id)
	}
	return &proxyHandler{
		info:      info,
		helper:    helper,
		settings:  settings,
		authMgr:   authMgr,
		whitelist: newDomainList(settings.DomainWhitelist),
		blacklist: newDomainList(settings.DomainBlacklist),
	}, nil
}

func (h *proxyHandler) Info() *handler.Info {
	return h.info
}

func (h *proxyHandler) Shutdown() {
	h.authMgr.Shutdown()
}

func isAuthEnabled(settings *config.AnyProxySettings) bool {
	return settings.Auth != nil && settings.Auth.Enabled
}

func (h *proxyHandler) isAllowedMethod(method string) bool {
	for _, m := range h.settings.AllowedMethods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

func (h *proxyHandler) isAllowedHost(host string) bool {
	if len(h.whitelist) > 0 && !h.whitelist.matches(host) {
		return false
	}
	if h.blacklist.matches(host) {
		return false
	}
	return h.settings.AllowPrivateNetwork || !utils.IsPrivateNetworkHost(host)
}

func checkPublicAddr(addr netip.AddrPort) error {
	if utils.IsPrivateNetworkAddr(addr.Addr()) {
		return common.NewHttpError(http.StatusForbidden, "Forbidden host")
	}
	return nil
}

func (h *proxyHandler) checkAuthenticate(w http.ResponseWriter, r *http.Request) bool {
	if !isAuthEnabled(h.settings) {
		return true
	}
	if user, pass, ok := r.BasicAuth(); ok && h.authMgr.Check(user, pass) {
		return true
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="pavonis"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return false
}

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	if !h.checkAuthenticate(w, r) {
		return
	}
	if !h.isAllowedMethod(r.Method) {
		w.Header().Set("Allow", strings.Join(h.settings.AllowedMethods, ", "))
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	reqPath, ok := utils.CutPathPrefix(r.URL.Path, h.info.PathPrefix)
	if !ok {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	target, httpErr := h.resolveTarget(reqPath, r.URL.RawQuery)
	if httpErr != nil {
		http.Error(w, httpErr.Message, httpErr.Status)
		return
	}
	if err := setUpstreamAuthorization(r, target); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.helper.RunReverseProxy(ctx, w, r, target,
		common.WithRedirectHandler(h.handleRedirect),
		common.WithResponseModifier(h.modifyResponse),
		common.WithRequestSizeLimit(h.settings.SizeLimit),
		common.WithResponseSizeLimit(h.settings.SizeLimit),
	)
}

// resolveTarget parses the target url from the request path with the site path prefix removed, and checks if its host is allowed.
// The userinfo of the target url is kept
func (h *proxyHandler) resolveTarget(reqPath, rawQuery string) (*url.URL, *common.HttpError) {
	target, err := utils.ParseHttpUrlWithDefaultScheme(strings.TrimPrefix(reqPath, "/"), "https")
	if err != nil || target.Scheme == "" || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return nil, common.NewHttpError(http.StatusBadRequest, "Invalid target URL")
	}
	if !h.isAllowedHost(target.Hostname()) {
		return nil, common.NewHttpError(http.StatusForbidden, "Forbidden host")
	}
	target.RawQuery = rawQuery
	return target, nil
}

func setUpstreamAuthorization(r *http.Request, target *url.URL) error {
	upstreamAuth := r.Header.Get(headerUpstreamAuthorization)
	r.Header.Del(headerUpstreamAuthorization)
	r.Header.Del("Authorization")

	if target.User != nil {
		if upstreamAuth != "" {
			return fmt.Errorf("upstream credentials are provided by both the URL and the %s header", headerUpstreamAuthorization)
		}
		password, _ := target.User.Password()
		r.SetBasicAuth(target.User.Username(), password)
		target.User = nil
	} else if upstreamAuth != "" {
		r.Header.Set("Authorization", upstreamAuth)
	}
	return nil
}

func (h *proxyHandler) handleRedirect(resp *http.Response) *common.RedirectResult {
	location, err := resp.Location()
	if err != nil {
		return &common.RedirectResult{Decision: common.RedirectDecisionReturn}
	}
	if (location.Scheme == "http" || location.Scheme == "https") && h.isAllowedHost(location.Hostname()) {
		return &common.RedirectResult{Decision: common.RedirectDecisionFollow}
	}
	return &common.RedirectResult{
		Decision: common.RedirectDecisionAbort,
		Err:      common.NewHttpError(http.StatusForbidden, fmt.Sprintf("Forbidden redirect target %s", location.Host)),
	}
}

func (h *proxyHandler) modifyResponse(_ *http.Request, resp *http.Response) error {
	if values := resp.Header.Values("WWW-Authenticate"); len(values) > 0 {
		resp.Header.Del("WWW-Authenticate")
		for _, value := range values {
			resp.Header.Add(headerUpstreamWwwAuthenticate, value)
		}
	}
	return nil
}
