package proxy

import (
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type ContainerRegistryHandler struct {
	helper   *common.RequestHelper
	settings *config.ContainerRegistrySettings
}

var _ http.Handler = &ContainerRegistryHandler{}

var realmPattern = regexp.MustCompile(`realm="[^"]+"`)

func NewContainerRegistryHandler(helper *common.RequestHelper, settings *config.ContainerRegistrySettings) *ContainerRegistryHandler {
	return &ContainerRegistryHandler{
		helper:   helper,
		settings: settings,
	}
}

func (h *ContainerRegistryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	var targetURL *url.URL
	var pathPrefix string
	if strings.HasPrefix(path, "/v2") {
		targetURL = h.settings.ParsedUpstreamV2Url
		pathPrefix = "/v2"
	} else if strings.HasPrefix(path, "/token") {
		targetURL = h.settings.ParsedUpstreamTokenUrl
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
		if strings.HasPrefix(resp.Request.URL.Path, "/v2") && resp.StatusCode == http.StatusUnauthorized {
			if auth, ok := resp.Header["Www-Authenticate"]; ok && len(auth) > 0 {
				newRealm := h.settings.SelfUrl + "/token"
				newAuth := realmPattern.ReplaceAllString(auth[0], `realm="`+newRealm+`"`)
				resp.Header.Set("Www-Authenticate", newAuth)
			}
		}
		return nil
	}

	h.helper.RunReverseProxy(w, r, &downstreamUrl, responseModifier)
}
