package server

import (
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/proxy"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"time"
)

type ProxyServer struct {
	handlers       map[string]http.Handler
	defaultHandler http.Handler
	helper         *common.RequestHelper
}

var _ http.Handler = &ProxyServer{}

func (s *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if hostPart, _, err := net.SplitHostPort(r.Host); err == nil {
		host = hostPart
	}
	clientAddr := utils.GetRequestClientIp(r)

	log.WithField("Host", host).Infof("%s - %s %s", clientAddr, r.Method, r.URL)
	startTime := time.Now()

	ww := utils.NewResponseWriterWrapper(w)
	if handler, ok := s.handlers[host]; ok {
		handler.ServeHTTP(ww, r)
	} else if s.defaultHandler != nil {
		s.defaultHandler.ServeHTTP(ww, r)
	} else {
		http.Error(ww, "Unknown host: "+host, http.StatusNotFound)
	}

	code := ww.GetStatusCode()
	costSec := time.Since(startTime).Seconds()
	log.Infof("%s - %s %s - %d %s %.3fs", clientAddr, r.Method, r.URL, code, http.StatusText(code), costSec)
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
