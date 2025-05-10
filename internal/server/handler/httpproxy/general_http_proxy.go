package httpproxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
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
	name     string
	helper   *common.RequestHelper
	mappings []*mapping
}

var _ handler.HttpHandler = &proxyHandler{}

func NewProxyHandler(name string, helper *common.RequestHelper, settings *config.HttpGeneralProxySettings) (handler.HttpHandler, error) {
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
		name:     name,
		helper:   helper,
		mappings: mappings,
	}, nil
}

func (h *proxyHandler) Name() string {
	return h.name
}

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	var mapping *mapping

	for _, m := range h.mappings {
		if strings.HasPrefix(path, m.PathPrefix) {
			mapping = m
			break
		}
	}
	if mapping == nil {
		http.Error(w, "Invalid path "+r.URL.Path, http.StatusNotFound)
		return
	}

	downstreamUrl := *r.URL
	downstreamUrl.Scheme = mapping.Destination.Scheme
	downstreamUrl.Host = mapping.Destination.Host
	downstreamUrl.Path = mapping.Destination.Path + r.URL.Path[len(mapping.PathPrefix):]
	h.helper.RunReverseProxy(ctx, w, r, &downstreamUrl, nil)
}
