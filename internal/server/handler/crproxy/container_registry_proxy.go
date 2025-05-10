package crproxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type proxyHandler struct {
	name     string
	helper   *common.RequestHelper
	settings *config.ContainerRegistrySettings

	upstreamV2Url    *url.URL
	upstreamTokenUrl *url.URL
}

var _ handler.HttpHandler = &proxyHandler{}

var realmPattern = regexp.MustCompile(`realm="[^"]+"`)

func NewContainerRegistryHandler(name string, helper *common.RequestHelper, settings *config.ContainerRegistrySettings) (handler.HttpHandler, error) {
	var err error
	var upstreamV2Url, upstreamTokenUrl *url.URL
	if upstreamV2Url, err = url.Parse(settings.UpstreamV2Url); err != nil {
		return nil, fmt.Errorf("invalid UpstreamV2Url %v: %v", settings.UpstreamV2Url, err)
	}
	if upstreamTokenUrl, err = url.Parse(settings.UpstreamTokenUrl); err != nil {
		return nil, fmt.Errorf("invalid upstreamTokenUrl %v: %v", settings.UpstreamTokenUrl, err)
	}

	return &proxyHandler{
		name:             name,
		helper:           helper,
		settings:         settings,
		upstreamV2Url:    upstreamV2Url,
		upstreamTokenUrl: upstreamTokenUrl,
	}, nil
}

func (h *proxyHandler) Name() string {
	return h.name
}

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Path
	if !strings.HasPrefix(reqPath, h.settings.PathPrefix) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	reqPath = reqPath[len(h.settings.PathPrefix):]

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

	downstreamUrl := *r.URL
	downstreamUrl.Scheme = targetURL.Scheme
	downstreamUrl.Host = targetURL.Host
	downstreamUrl.Path = targetURL.Path + reqPath[len(pathPrefix):]

	responseModifier := func(resp *http.Response) error {
		if pathPrefix == "/v2" && resp.StatusCode == http.StatusUnauthorized {
			if auth, ok := resp.Header["Www-Authenticate"]; ok && len(auth) > 0 {
				newRealm := h.settings.SelfUrl + "/token"
				newAuth := realmPattern.ReplaceAllString(auth[0], `realm="`+newRealm+`"`)
				resp.Header.Set("Www-Authenticate", newAuth)
			}
		}
		return nil
	}

	h.helper.RunReverseProxy(ctx, w, r, &downstreamUrl, responseModifier)
}
