package server

import (
	gocontext "context"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/felixge/httpsnoop"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type PavonisServer struct {
	cfg                *config.Config
	trustedProxiesPool *utils.IpPool
	trustedProxiesAll  bool
	allHandlers        []handler.HttpHandler
	handlersByHost     map[string][]handler.HttpHandler
	handlersDefault    []handler.HttpHandler
	shutdownFunctions  []func()
}

var _ http.Handler = &PavonisServer{}

func (s *PavonisServer) createRequestContext(r *http.Request) *context.RequestContext {
	host := r.Host
	if hostPart, _, err := net.SplitHostPort(r.Host); err == nil {
		host = hostPart
	}

	clientIp, clientAddr := utils.GetIpFromHostPort(r.RemoteAddr)
	if clientIp != nil && (s.trustedProxiesAll || s.trustedProxiesPool.Contains(clientIp)) {
		if realClientIp, ok := utils.GetRequestClientIpFromProxyHeader(r, *s.cfg.Server.TrustedProxyHeaders); ok {
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

func isWebSocketRequest(r *http.Request) bool {
	return r.Header.Get("Connection") == "Upgrade" && r.Header.Get("Upgrade") == "websocket"
}

func NewPavonisServer(cfg *config.Config) (*PavonisServer, error) {
	trustedProxiesAll := slices.Contains(*cfg.Server.TrustedProxyIps, "*")
	var trustedProxies *utils.IpPool
	if !trustedProxiesAll {
		var err error
		if trustedProxies, err = utils.NewIpPool(*cfg.Server.TrustedProxyIps); err != nil {
			return nil, fmt.Errorf("TrustedProxyIps IP pool init failed: %v", err)
		}
	}

	helperFactory, err := common.NewRequestHelperFactory(cfg)
	if err != nil {
		return nil, err
	}

	server := &PavonisServer{
		cfg:                cfg,
		trustedProxiesPool: trustedProxies,
		trustedProxiesAll:  trustedProxiesAll,
		handlersByHost:     make(map[string][]handler.HttpHandler),
	}
	server.shutdownFunctions = append(server.shutdownFunctions, helperFactory.Shutdown)

	for sideIdx, siteCfg := range cfg.Sites {
		siteInfo := handler.NewSiteInfo(siteCfg.Id, siteCfg)
		helper := helperFactory.NewRequestHelper(siteCfg.IpPoolStrategy)

		hdl, err := createSiteHttpHandler(*siteCfg.Mode, siteInfo, helper, siteCfg.Settings)
		if err != nil {
			return nil, fmt.Errorf("init site handler %d failed: %v", sideIdx, err)
		}

		server.allHandlers = append(server.allHandlers, hdl)
		if siteCfg.Host.IsWildcard() {
			server.handlersDefault = append(server.handlersDefault, hdl)
		} else {
			for _, host := range siteCfg.Host {
				server.handlersByHost[host] = append(server.handlersByHost[host], hdl)
			}
		}
	}

	sortHandlers := func(handlers []handler.HttpHandler) {
		sort.Slice(handlers, func(i, j int) bool {
			return len(handlers[i].Info().PathPrefix) > len(handlers[j].Info().PathPrefix)
		})
	}
	sortHandlers(server.handlersDefault)
	for _, handlers := range server.handlersByHost {
		sortHandlers(handlers)
	}

	return server, nil
}

func (s *PavonisServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// init
	ctx := s.createRequestContext(r)
	targetHandler := s.selectHandler(ctx.Host, r.URL.Path) // result might be nil
	handlerNamePrefix := ""
	if targetHandler != nil {
		handlerNamePrefix += targetHandler.Info().Id + ":"
	}
	ctx.LogPrefix = fmt.Sprintf("(%s%s) ", handlerNamePrefix, ctx.RequestId)

	// start logging
	logLine := ctx.LogPrefix + fmt.Sprintf("%s - %s %s", ctx.ClientAddr, r.Method, r.URL.Path)
	log.
		WithField("Host", ctx.Host).
		WithField("UA", sll(r.UserAgent(), 24)).
		Debug(logLine)

	// set total timeout for this request
	timeoutCtx, cancel := gocontext.WithTimeout(r.Context(), *s.cfg.ResourceLimit.RequestTimeout)
	defer cancel()
	r = r.WithContext(timeoutCtx)

	// http metrics
	hm := httpsnoop.Metrics{Code: http.StatusOK}

	defer func() {
		// end logging
		panicErr := recover()
		if panicErr == http.ErrAbortHandler {
			logLine += " (aborted)"
		} else if panicErr != nil {
			logLine += " (panic)"
		}

		log.
			WithField("Cost", fmt.Sprintf("%.3fs", time.Since(ctx.StartTime).Seconds())).
			WithField("Status", fmt.Sprintf("%d %s", hm.Code, http.StatusText(hm.Code))).
			Info(logLine)

		metricRequestServed.WithLabelValues(strconv.Itoa(hm.Code)).Inc()

		if panicErr != nil {
			panic(panicErr)
		}
	}()

	// process the request
	hm.CaptureMetrics(w, func(w http.ResponseWriter) {
		if isWebSocketRequest(r) {
			http.Error(w, "WebSocket connections are not allowed", http.StatusForbidden)
		}

		if targetHandler != nil {
			targetHandler.ServeHttp(ctx, w, r)
		} else {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	})
}

func (s *PavonisServer) Shutdown() {
	for _, hdl := range s.allHandlers {
		hdl.Shutdown()
	}
	for _, f := range s.shutdownFunctions {
		f()
	}
}

func (s *PavonisServer) selectHandler(host string, path string) handler.HttpHandler {
	var candidateHandlers []handler.HttpHandler
	if handlers, ok := s.handlersByHost[host]; ok {
		candidateHandlers = handlers
	} else if s.handlersDefault != nil {
		candidateHandlers = s.handlersDefault
	} else {
		return nil
	}

	for _, candidateHandler := range candidateHandlers {
		if strings.HasPrefix(path, candidateHandler.Info().PathPrefix) {
			return candidateHandler
		}
	}
	return nil
}

func (s *PavonisServer) NewHttpServer() *http.Server {
	return &http.Server{
		Addr:    *s.cfg.Server.Listen,
		Handler: s,
	}
}
