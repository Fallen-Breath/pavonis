package crproxy

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type routePrefix string

const (
	routePrefixV1        routePrefix = "/v1"
	routePrefixV2        routePrefix = "/v2"
	routePrefixAuthRealm routePrefix = "/auth"
)

var v1ListRepositoryTagsPathPattern = regexp.MustCompile(`^/v1/repositories/.+/tags$`)

func (h *proxyHandler) getRoute(w http.ResponseWriter, reqPath string) (targetUrl *url.URL, routePrefix routePrefix, ok bool) {
	ok = true
	if h.upstreamV1Url != nil && strings.HasPrefix(reqPath, string(routePrefixV1)) {
		targetUrl = h.upstreamV1Url
		routePrefix = routePrefixV1

		// reason for supporting v1: docker client still uses the /v1 endpoint for its search command
		// we only allow list operation in v1 registry
		//
		// V1 APIs (incomplete, but are enough for listing)
		// GET      /v1/_ping
		// GET      /v1/search
		// GET      /v1/repositories/<name>/tags
		if reqPath != "/v1/_ping" && reqPath != "/v1/search" && !v1ListRepositoryTagsPathPattern.MatchString(reqPath) {
			ok = false
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	} else if strings.HasPrefix(reqPath, string(routePrefixV2)) {
		targetUrl = h.upstreamV2Url
		routePrefix = routePrefixV2
	} else if strings.HasPrefix(reqPath, string(routePrefixAuthRealm)) {
		targetUrl = h.upstreamAuthRealmUrl
		routePrefix = routePrefixAuthRealm
	} else {
		ok = false
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
	return
}
