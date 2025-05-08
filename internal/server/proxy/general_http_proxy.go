package proxy

import (
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type HttpProxyMapping struct {
	PathPrefix  string
	Destination *url.URL
}

type HttpGeneralProxyHandler struct {
	helper   *common.RequestHelper
	mappings []*HttpProxyMapping
}

var _ http.Handler = &HttpGeneralProxyHandler{}

func (h *HttpGeneralProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	var mapping *HttpProxyMapping

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
	h.helper.RunReverseProxy(w, r, &downstreamUrl, nil)
}

func NewHttpGeneralProxyHandler(helper *common.RequestHelper, settings *config.HttpGeneralProxySettings) *HttpGeneralProxyHandler {
	var mappings []*HttpProxyMapping

	addMapping := func(pathPrefix, destination string) {
		destURL, err := url.Parse(destination)
		if err != nil {
			log.Fatalf("invalid destination URL %s: %v", pathPrefix, err)
		}
		if destURL.Scheme == "" || destURL.Host == "" {
			log.Fatalf("invalid destination URL %s", pathPrefix)
		}
		mappings = append(mappings, &HttpProxyMapping{
			PathPrefix:  pathPrefix,
			Destination: destURL,
		})
	}

	if settings.Destination != "" {
		addMapping("", settings.Destination)
	}
	for _, m := range settings.Mappings {
		addMapping(m.Path, m.Destination)
	}

	sort.Slice(mappings, func(i, j int) bool {
		return len(mappings[i].PathPrefix) > len(mappings[j].PathPrefix)
	})
	return &HttpGeneralProxyHandler{
		helper:   helper,
		mappings: mappings,
	}
}
