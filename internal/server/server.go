package server

import (
	gocontext "context"
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

type PavonisServer struct {
	cfg               *config.Config
	trustedProxies    *utils.IpPool
	trustedProxiesAll bool
	handlers          map[string]proxy.HttpHandler
	defaultHandler    proxy.HttpHandler
	shutdownFunctions []func()
}

var _ http.Handler = &PavonisServer{}

func (s *PavonisServer) createRequestContext(r *http.Request) *context.RequestContext {
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
	return context.NewRequestContext(host, clientAddr)
}

func sll(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit-3] + "..."
}

func NewPavonisServer(cfg *config.Config) (*PavonisServer, error) {
	trustedProxiesAll := *cfg.Server.TrustedProxies == "*"
	var trustedProxies *utils.IpPool
	if !trustedProxiesAll {
		var err error
		if trustedProxies, err = utils.NewIpPool(strings.Split(*cfg.Server.TrustedProxies, ",")); err != nil {
			return nil, fmt.Errorf("TrustedProxies IP pool init failed: %v", err)
		}
	}

	helperFactory, err := common.NewRequestHelperFactory(cfg)
	if err != nil {
		return nil, err
	}

	server := &PavonisServer{
		cfg:               cfg,
		trustedProxies:    trustedProxies,
		trustedProxiesAll: trustedProxiesAll,
		handlers:          make(map[string]proxy.HttpHandler),
	}
	server.shutdownFunctions = append(server.shutdownFunctions, helperFactory.Shutdown)

	for sideIdx, site := range cfg.Sites {
		siteName := fmt.Sprintf("site%d", sideIdx)
		helper := helperFactory.NewRequestHelper(site.IpPoolStrategy)

		var err error
		var handler proxy.HttpHandler
		switch site.Mode {
		case config.HttpGeneralProxy:
			handler, err = proxy.NewHttpGeneralProxyHandler(siteName, helper, site.Settings.(*config.HttpGeneralProxySettings))
		case config.GithubDownloadProxy:
			handler, err = proxy.NewGithubProxyHandler(siteName, helper, site.Settings.(*config.GithubDownloadProxySettings))
		case config.ContainerRegistryProxy:
			handler, err = proxy.NewContainerRegistryHandler(siteName, helper, site.Settings.(*config.ContainerRegistrySettings))
		case config.PypiProxy:
			handler, err = proxy.NewPypiHandler(siteName, helper, site.Settings.(*config.PypiRegistrySettings))

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

func (s *PavonisServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// context init
	ctx := s.createRequestContext(r)
	var handler proxy.HttpHandler
	if hdl, ok := s.handlers[ctx.Host]; ok {
		handler = hdl
	} else if s.defaultHandler != nil {
		handler = s.defaultHandler
	}
	handlerNamePrefix := ""
	if handler != nil {
		handlerNamePrefix = handler.Name() + ":"
	}
	ctx.LogPrefix = fmt.Sprintf("[%s%s] ", handlerNamePrefix, ctx.RequestId)

	// start logging
	logLine := ctx.LogPrefix + fmt.Sprintf("%s - %s %s", ctx.ClientAddr, r.Method, r.URL)
	log.
		WithField("Host", ctx.Host).
		WithField("UA", sll(r.UserAgent(), 24)).
		Debug(logLine)

	// set total timeout for this request
	requestTimeout := 1 * time.Hour
	if s.cfg.ResourceLimit.RequestTimeout != nil {
		requestTimeout = *s.cfg.ResourceLimit.RequestTimeout
	}
	timeoutCtx, cancel := gocontext.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	r = r.WithContext(timeoutCtx)

	writerWrapper := utils.NewResponseWriterWrapper(w)

	defer func() {
		// end logging
		panicErr := recover()
		if panicErr == http.ErrAbortHandler {
			logLine += " (aborted)"
		} else if panicErr != nil {
			logLine += " (panic)"
		}

		costSec := time.Since(ctx.StartTime).Seconds()
		var statusText string
		if code := writerWrapper.GetStatusCode(); code != nil {
			statusText = fmt.Sprintf("%d %s", *code, http.StatusText(*code))
		} else {
			statusText = "N/A"
		}
		log.
			WithField("Cost", fmt.Sprintf("%.3fs", costSec)).
			WithField("Status", statusText).
			Info(logLine)

		if panicErr != nil {
			panic(panicErr)
		}
	}()

	// process the request
	if handler != nil {
		handler.ServeHttp(ctx, writerWrapper, r)
	} else {
		http.Error(writerWrapper, "Unknown host: "+ctx.Host, http.StatusNotFound)
	}
}

func (s *PavonisServer) Shutdown() {
	for _, f := range s.shutdownFunctions {
		f()
	}
}
