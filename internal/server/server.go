package server

import (
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/proxy"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
)

type ProxyServer struct {
	handlers       map[string]http.Handler
	defaultHandler http.Handler
	helper         *common.RequestHelper
}

var _ http.Handler = &ProxyServer{}

func (s *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if hostPart, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = hostPart
	}
	log.WithField("Host", host).Infof("%s - %s %s", r.RemoteAddr, r.Method, r.URL)

	if handler, ok := s.handlers[host]; ok {
		handler.ServeHTTP(w, r)
	} else if s.defaultHandler != nil {
		s.defaultHandler.ServeHTTP(w, r)
	} else {
		http.Error(w, "Unknown host: "+host, http.StatusNotFound)
	}
}

func NewServer(cfg *config.Config) *ProxyServer {
	helper := common.NewRequestHelper(cfg)
	server := &ProxyServer{
		handlers: make(map[string]http.Handler),
		helper:   helper,
	}

	for _, site := range cfg.Sites {
		var handler http.Handler
		switch site.Mode {
		case config.HttpGeneralProxy:
			handler = proxy.NewHttpGeneralProxyHandler(helper, site.Settings.(*config.HttpGeneralProxySettings))

		case config.GithubDownloadProxy:
			handler = proxy.NewGithubProxyHandler(helper, site.Settings.(*config.GithubDownloadProxySettings))

		case config.ContainerRegistryProxy:
			handler = proxy.NewContainerRegistryHandler(helper, site.Settings.(*config.ContainerRegistrySettings))

		default:
			log.Fatalf("Unknown mode %s for host %s", site.Mode, site.Host)
		}

		if site.Host == "*" {
			server.defaultHandler = handler
		} else {
			server.handlers[site.Host] = handler
		}
	}

	return server
}

func (s *ProxyServer) Shutdown() {
	s.helper.Shutdown()
}
