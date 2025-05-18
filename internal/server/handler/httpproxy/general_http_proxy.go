package httpproxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
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
	if !strings.HasPrefix(r.URL.Path, h.info.PathPrefix) {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, h.info.PathPrefix))
	}
	reqPath := r.URL.Path[len(h.info.PathPrefix):]

	var mapping *mapping

	for _, m := range h.mappings {
		if strings.HasPrefix(reqPath, m.PathPrefix) {
			mapping = m
			break
		}
	}
	if mapping == nil {
		http.Error(w, "Invalid path "+reqPath, http.StatusNotFound)
		return
	}

	downstreamUrl := *r.URL
	downstreamUrl.Scheme = mapping.Destination.Scheme
	downstreamUrl.Host = mapping.Destination.Host
	downstreamUrl.Path = mapping.Destination.Path + reqPath[len(mapping.PathPrefix):]

	redirectAction := *h.settings.RedirectAction
	rpOpt := common.WithRedirectAction(redirectAction, func(resp *http.Response) *string {
		// rewrite relative url, i.e. rewrite iff location is under downstreamUrl
		if location, err := resp.Location(); err == nil && location != nil {
			// client -> pavonis -> downstream
			// https://downstream.com/mappingDestinationPath/downstream/foo/bar (downstreamUrl, downstream)
			//                       [        srcPath      ]
			// https://pavonis.server/pathPrefix/mappingPrefix/downstream/foo/bar (r.URL, upstream)
			//                       [        dstPath        ]
			srcPath := mapping.Destination.Path
			dstPath := r.URL.Path[:len(h.info.PathPrefix)+len(mapping.PathPrefix)]
			if (location.Scheme == downstreamUrl.Scheme && location.Host == downstreamUrl.Host) || (location.Scheme == "" && location.Host == "") {
				if strings.HasPrefix(location.Path, srcPath) {
					oldLocation := location.String()
					location.Path = dstPath + location.Path[len(srcPath):]
					newLocation := location.String()
					log.Debugf("%sRewriting redirect response (%s) Location from %+q to %+q", ctx.LogPrefix, resp.Status, oldLocation, newLocation)
					return &newLocation
				}
			}
		}
		return nil
	})

	h.helper.RunReverseProxy(ctx, w, r, &downstreamUrl, rpOpt)
}
