package anyproxy

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
)

type proxyHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.AnyProxySettings
}

var _ handler.HttpHandler = (*proxyHandler)(nil)

func NewHandler(info *handler.Info, helper *common.RequestHelper, settings *config.AnyProxySettings) (handler.HttpHandler, error) {
	return &proxyHandler{info: info, helper: helper, settings: settings}, nil
}
func (h *proxyHandler) Info() *handler.Info {
	return h.info
}

func (h *proxyHandler) Shutdown() {

}

func (h *proxyHandler) isAllowedMethod(method string) bool {
	methods := h.settings.AllowedMethods
	if len(methods) == 0 {
		return method == http.MethodGet
	}
	for _, m := range methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

func (h *proxyHandler) isBlacklisted(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, rule := range h.settings.DomainBlacklist {
		rule = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(rule), "."))
		if rule == "" {
			continue
		}
		if strings.HasPrefix(rule, "*.") {
			if strings.HasSuffix(host, rule[1:]) && host != rule[2:] {
				return true
			}
		} else if host == rule {
			return true
		}
	}
	return false
}

func (h *proxyHandler) checkAuthenticate(w http.ResponseWriter, r *http.Request) bool {
	if h.settings.Auth == nil || !h.settings.Auth.Enabled {
		return true
	}
	user, pass, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="pavonis"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	for _, u := range h.settings.Auth.Users {
		if u != nil && u.Name == user && u.Password == pass {
			return true
		}
	}
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return false
}

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	if !h.checkAuthenticate(w, r) {
		return
	}
	if !h.isAllowedMethod(r.Method) {
		if len(h.settings.AllowedMethods) > 0 {
			w.Header().Set("Allow", strings.Join(h.settings.AllowedMethods, ", "))
		}
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if !strings.HasPrefix(r.URL.Path, h.info.PathPrefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	raw := strings.TrimPrefix(r.URL.Path[len(h.info.PathPrefix):], "/")
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	target, err := url.Parse(raw)
	if err != nil || target.Scheme == "" || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return
	}

	if h.isBlacklisted(target.Hostname()) {
		http.Error(w, "Forbidden host", http.StatusForbidden)
		return
	}
	target.RawQuery = r.URL.RawQuery
	if target.User != nil {
		user := target.User.Username()
		pass, has := target.User.Password()
		if has {
			r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
		} else {
			r.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":")))
		}
		target.User = nil
	} else {
		r.Header.Del("Authorization")
	}

	h.helper.RunReverseProxy(ctx, w, r, target)
}
