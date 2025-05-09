package proxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type ContainerRegistryHandler struct {
	helper   *common.RequestHelper
	settings *config.ContainerRegistrySettings

	upstreamV2Url    *url.URL
	upstreamTokenUrl *url.URL
}

var _ HttpHandler = &ContainerRegistryHandler{}

var realmPattern = regexp.MustCompile(`realm="[^"]+"`)

func NewContainerRegistryHandler(helper *common.RequestHelper, settings *config.ContainerRegistrySettings) (*ContainerRegistryHandler, error) {
	var err error
	var upstreamV2Url, upstreamTokenUrl *url.URL
	if upstreamV2Url, err = url.Parse(settings.UpstreamV2Url); err != nil {
		return nil, fmt.Errorf("invalid UpstreamV2Url %v: %v", settings.UpstreamV2Url, err)
	}
	if upstreamTokenUrl, err = url.Parse(settings.UpstreamTokenUrl); err != nil {
		return nil, fmt.Errorf("invalid upstreamTokenUrl %v: %v", settings.UpstreamTokenUrl, err)
	}

	return &ContainerRegistryHandler{
		helper:           helper,
		settings:         settings,
		upstreamV2Url:    upstreamV2Url,
		upstreamTokenUrl: upstreamTokenUrl,
	}, nil
}

func (h *ContainerRegistryHandler) ServeHttp(ctx *context.HttpContext, w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if !strings.HasPrefix(path, h.settings.PathPrefix) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	path = path[len(h.settings.PathPrefix):]

	var targetURL *url.URL
	var pathPrefix string
	if strings.HasPrefix(path, "/v2") {
		targetURL = h.upstreamV2Url
		pathPrefix = "/v2"
	} else if strings.HasPrefix(path, "/token") {
		targetURL = h.upstreamTokenUrl
		pathPrefix = "/token"
	} else {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	downstreamUrl := *r.URL
	downstreamUrl.Scheme = targetURL.Scheme
	downstreamUrl.Host = targetURL.Host
	downstreamUrl.Path = targetURL.Path + r.URL.Path[len(pathPrefix):]

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
