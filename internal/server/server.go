package server

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/proxy"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"strings"
	"time"
)

type ProxyServer struct {
	cfg               *config.Config
	trustedProxies    *utils.IpPool
	trustedProxiesAll bool
	handlers          map[string]proxy.HttpHandler
	defaultHandler    proxy.HttpHandler
	shutdownFunctions []func()
}

var _ http.Handler = &ProxyServer{}

func (s *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	host := r.Host
	if hostPart, _, err := net.SplitHostPort(r.Host); err == nil {
		host = hostPart
	}

	clientIp, clientAddr := utils.GetIpFromHostPort(r.RemoteAddr)
	if clientIp != nil && s.trustedProxies.Contains(clientIp) {
		if realClientIp, ok := utils.GetRequestClientIpFromProxyHeader(r); ok {
			clientAddr = realClientIp
		}
	}
	ctx := context.NewHttpContext(clientAddr)

	ww := utils.NewResponseWriterWrapper(w)
	if handler, ok := s.handlers[host]; ok {
		handler.ServeHttp(ctx, ww, r)
	} else if s.defaultHandler != nil {
		s.defaultHandler.ServeHttp(ctx, ww, r)
	} else {
		http.Error(ww, "Unknown host: "+host, http.StatusNotFound)
	}

	code := ww.GetStatusCode()
	costSec := time.Since(startTime).Seconds()
	log.Infof("[%s] %s - %s %s - %d %s %.3fs", host, clientAddr, r.Method, r.URL, code, http.StatusText(code), costSec)
}

func NewServer(cfg *config.Config) (*ProxyServer, error) {
	trustedProxiesAll := *cfg.Server.TrustedProxies == "*"
	var trustedProxies *utils.IpPool
	if !trustedProxiesAll {
		var err error
		if trustedProxies, err = utils.NewIpPool(strings.Split(*cfg.Server.TrustedProxies, ",")); err != nil {
			return nil, fmt.Errorf("TrustedProxies IP pool init failed: %v", err)
		}
	}

	helperFactory := common.NewRequestHelperFactory(cfg)
	server := &ProxyServer{
		cfg:               cfg,
		trustedProxies:    trustedProxies,
		trustedProxiesAll: trustedProxiesAll,
		handlers:          make(map[string]proxy.HttpHandler),
	}
	server.shutdownFunctions = append(server.shutdownFunctions, helperFactory.Shutdown)

	for sideIdx, site := range cfg.Sites {
		helper := helperFactory.NewRequestHelper(site.IpPoolStrategy)

		var err error
		var handler proxy.HttpHandler
		switch site.Mode {
		case config.HttpGeneralProxy:
			handler, err = proxy.NewHttpGeneralProxyHandler(helper, site.Settings.(*config.HttpGeneralProxySettings))
		case config.GithubDownloadProxy:
			handler, err = proxy.NewGithubProxyHandler(helper, site.Settings.(*config.GithubDownloadProxySettings))
		case config.ContainerRegistryProxy:
			handler, err = proxy.NewContainerRegistryHandler(helper, site.Settings.(*config.ContainerRegistrySettings))

		default:
			err = fmt.Errorf("unknown mode %s", site.Mode)
		}
		if err != nil {
			return nil, fmt.Errorf("init site handler %d failed: %v", sideIdx, err)
		}

		if site.Host == "*" {
			server.defaultHandler = handler
		} else {
			server.handlers[site.Host] = handler
		}
	}

	return server, nil
}

func (s *ProxyServer) Shutdown() {
	for _, f := range s.shutdownFunctions {
		f()
	}
}
