package httpproxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type mapping struct {
	PathPrefix  string
	Destination *url.URL
}

type proxyHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.HttpGeneralProxySettings

	mappings []*mapping
}

var _ handler.HttpHandler = &proxyHandler{}

func NewProxyHandler(info *handler.Info, helper *common.RequestHelper, settings *config.HttpGeneralProxySettings) (handler.HttpHandler, error) {
	var mappings []*mapping

	addMapping := func(pathPrefix, destination string) error {
		destURL, err := url.Parse(destination)
		if err != nil {
			return fmt.Errorf("invalid destination URL %s: %v", pathPrefix, err)
		}
		if destURL.Scheme == "" || destURL.Host == "" {
			return fmt.Errorf("invalid destination URL %s", pathPrefix)
		}
		mappings = append(mappings, &mapping{
			PathPrefix:  pathPrefix,
			Destination: destURL,
		})
		return nil
	}
	if settings.Destination != "" {
		if err := addMapping("", settings.Destination); err != nil {
			return nil, err
		}
	}
	for _, m := range settings.Mappings {
		if err := addMapping(m.Path, m.Destination); err != nil {
			return nil, err
		}
	}
	sort.Slice(mappings, func(i, j int) bool {
		return len(mappings[i].PathPrefix) > len(mappings[j].PathPrefix)
	})

	return &proxyHandler{
		info:     info,
		helper:   helper,
		settings: settings,

		mappings: mappings,
	}, nil
}

func (h *proxyHandler) Info() *handler.Info {
	return h.info
}

func (h *proxyHandler) Shutdown() {
}

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	reqPath, ok := utils.CutPathPrefix(r.URL.Path, h.info.PathPrefix)
	if !ok {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, h.info.PathPrefix))
	}

	var mapping *mapping
	var mappedPath string
	for _, m := range h.mappings {
		if rest, ok := utils.CutPathPrefix(reqPath, m.PathPrefix); ok {
			mapping = m
			mappedPath = rest
			break
		}
	}
	if mapping == nil {
		http.Error(w, "Invalid path "+reqPath, http.StatusNotFound)
		return
	}

	upstreamUrl := *r.URL
	upstreamUrl.Scheme = mapping.Destination.Scheme
	upstreamUrl.Host = mapping.Destination.Host
	upstreamUrl.Path = mapping.Destination.Path + mappedPath

	h.helper.RunReverseProxy(
		ctx, w, r, &upstreamUrl,
		common.WithRedirectAction(*h.settings.RedirectAction, func(resp *http.Response) *string {
			return h.rewriteLocation(ctx, mapping, resp)
		}),
	)
}

// rewriteLocation maps a Location under the mapping destination back to this site
func (h *proxyHandler) rewriteLocation(ctx *context.RequestContext, mapping *mapping, resp *http.Response) *string {
	location, err := resp.Location()
	if err != nil || location == nil {
		return nil
	}
	sameOrigin := location.Scheme == mapping.Destination.Scheme && location.Host == mapping.Destination.Host
	if !sameOrigin && (location.Scheme != "" || location.Host != "") {
		return nil
	}
	rest, ok := utils.CutPathPrefix(location.Path, strings.TrimSuffix(mapping.Destination.Path, "/"))
	if !ok {
		return nil
	}

	oldLocation := location.String()
	// Use relative url here, so we don't need to use self-url's Scheme / Host
	// reference: https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Location
	// XXX: handle if the user has configured another upper-level reversed proxy
	location.Scheme = ""
	location.Host = ""
	location.Path = h.info.PathPrefix + mapping.PathPrefix + rest
	newLocation := location.String()
	log.Debugf("%sRewriting redirect response (%s) Location from %+q to %+q", ctx.LogPrefix, resp.Status, oldLocation, newLocation)
	return &newLocation
}
