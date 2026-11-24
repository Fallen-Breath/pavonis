package server

import (
	gocontext "context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/server/limits"
	"github.com/Fallen-Breath/pavonis/internal/server/metrics"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
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
	var clientPort uint16
	if remoteAddr, err := netip.ParseAddrPort(r.RemoteAddr); err == nil {
		clientPort = remoteAddr.Port()
	}
	if clientIp != nil && s.isTrustedProxy(clientIp) {
		if realClientIp, ok := utils.GetRequestClientIpFromProxyHeader(r, *s.cfg.Server.TrustedProxyHeaders, s.isTrustedProxy); ok {
			clientAddr = realClientIp
			clientPort = 0
		}
	}

	ctx := context.NewRequestContext(host, clientAddr)
	ctx.ClientPort = clientPort
	if localAddr, ok := r.Context().Value(http.LocalAddrContextKey).(*net.TCPAddr); ok {
		ctx.ClientLocalAddr = localAddr.AddrPort()
	}
	return ctx
}

func (s *PavonisServer) isTrustedProxy(ip net.IP) bool {
	return s.trustedProxiesAll || s.trustedProxiesPool.Contains(ip)
}

func sll(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit-3] + "..."
}

func isWebSocketRequest(r *http.Request) bool {
	// RFC 9110 sections 7.6.1 and 7.8 define these fields as comma-separated lists.
	// RFC 6455 section 4.2.1 requires case-insensitive matching of the handshake values.
	headerContainsToken := func(headerName, token string) bool {
		for _, value := range r.Header.Values(headerName) {
			for _, part := range strings.Split(value, ",") {
				if strings.EqualFold(strings.Trim(part, " \t"), token) {
					return true
				}
			}
		}
		return false
	}
	return headerContainsToken("Connection", "upgrade") && headerContainsToken("Upgrade", "websocket")
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

	limitManager, err := limits.NewManager(cfg.ResourceLimit)
	if err != nil {
		server.Shutdown()
		return nil, fmt.Errorf("init resource limits failed: %v", err)
	}
	server.shutdownFunctions = append(server.shutdownFunctions, limitManager.Shutdown)

	for sideIdx, siteCfg := range cfg.Sites {
		helper := helperFactory.NewRequestHelper(siteCfg.Id, siteCfg.IpPoolStrategy)

		hdl, err := createSiteHandler(cfg, siteCfg, helper, limitManager)
		if err != nil {
			server.Shutdown()
			return nil, fmt.Errorf("init site handler %d failed: %v", sideIdx, err)
		}

		server.allHandlers = append(server.allHandlers, hdl)
		if siteCfg.Host.IsWildcard() {
			server.handlersDefault = append(server.handlersDefault, hdl)
		} else {
			for _, host := range siteCfg.Host {
				host = strings.ToLower(host)
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

	if cfg.Diagnostics.Enabled {
		unregisterMetrics, err := metrics.RegisterRuntimeMetrics(helperFactory.ReadRequestProxyBlockStates)
		if err != nil {
			server.Shutdown()
			return nil, fmt.Errorf("register metrics: %w", err)
		}
		server.shutdownFunctions = append(server.shutdownFunctions, unregisterMetrics)
	}
	return server, nil
}

func (s *PavonisServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// init
	ctx := s.createRequestContext(r)
	escapedPath := r.URL.EscapedPath()
	targetHandler := s.selectHandler(ctx.Host, escapedPath) // result might be nil
	targetHandlerId := ""
	handlerNamePrefix := ""
	timeout := *s.cfg.ResourceLimit.Global.RequestTimeout
	if targetHandler != nil {
		timeout = targetHandler.Info().RequestTimeout
		targetHandlerId = targetHandler.Info().Id
		handlerNamePrefix = targetHandlerId + ":"
	}
	ctx.LogPrefix = fmt.Sprintf("(%s%s) ", handlerNamePrefix, ctx.RequestId)

	// start logging
	logLine := ctx.LogPrefix + fmt.Sprintf("%s - %s %s", ctx.ClientAddr, r.Method, escapedPath)
	log.
		WithField("Host", ctx.Host).
		WithField("UA", sll(r.UserAgent(), 24)).
		Debug(logLine)

	// adjust request context
	rContext := r.Context()
	rContext = gocontext.WithValue(rContext, context.Key, ctx)
	rContext = utils.WithProxyAttemptHook(rContext, func(id string) { ctx.RequestProxyId = id })
	reqContext, cancel := gocontext.WithTimeout(rContext, timeout) // total timeout
	defer cancel()
	r = r.WithContext(reqContext)
	deadline, _ := reqContext.Deadline()
	controller := http.NewResponseController(w)
	if r.Body != nil && r.Body != http.NoBody {
		if err := controller.SetReadDeadline(deadline); err != nil {
			log.Warnf("%sFailed to set request body read deadline: %v", ctx.LogPrefix, err)
		}
	}
	if err := controller.SetWriteDeadline(deadline); err != nil {
		log.Warnf("%sFailed to set response write deadline: %v", ctx.LogPrefix, err)
	}

	// http metrics
	var response responseMetrics
	w = response.wrap(w)
	finishMetrics := metrics.TrackServerRequest(r, targetHandlerId)

	defer func() {
		// end logging
		panicErr := recover()
		if panicErr == http.ErrAbortHandler {
			if ctx.AbortReason != "" {
				logLine += " (aborted: " + ctx.AbortReason + ")"
			} else {
				logLine += " (aborted)"
			}
		} else if panicErr != nil {
			logLine += " (panic)"
		}

		costSec := time.Since(ctx.StartTime).Seconds()
		statusCode := response.status
		if statusCode == 0 && panicErr == nil {
			statusCode = http.StatusOK
		}
		status := "-"
		if statusCode != 0 {
			status = fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode))
		}
		entry := log.
			WithField("Cost", fmt.Sprintf("%.3fs", costSec)).
			WithField("Status", status)
		if ctx.RequestProxyId != "" {
			entry = entry.WithField("Proxy", ctx.RequestProxyId)
		}
		entry.Info(logLine)

		finishMetrics(statusCode, costSec, response.written)

		if panicErr != nil {
			panic(panicErr)
		}
	}()

	// process the request
	if isWebSocketRequest(r) {
		http.Error(w, "WebSocket connections are not allowed", http.StatusForbidden)
		return
	}
	if targetHandler == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	targetHandler.ServeHttp(ctx, w, r)
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
	if handlers, ok := s.handlersByHost[strings.ToLower(host)]; ok {
		candidateHandlers = handlers
	} else if s.handlersDefault != nil {
		candidateHandlers = s.handlersDefault
	} else {
		return nil
	}

	for _, candidateHandler := range candidateHandlers {
		if _, ok := utils.CutPathPrefix(path, candidateHandler.Info().PathPrefix); ok {
			return candidateHandler
		}
	}
	return nil
}

type HTTPServer struct {
	server         *http.Server
	maxConnections int
}

func (s *PavonisServer) CreateHttpServer() *HTTPServer {
	return &HTTPServer{
		server: &http.Server{
			Addr:              *s.cfg.Server.Listen,
			Handler:           s,
			ReadHeaderTimeout: *s.cfg.Server.ReadHeaderTimeout,
			IdleTimeout:       *s.cfg.Server.IdleTimeout,
		},
		maxConnections: *s.cfg.Server.MaxConnections,
	}
}

func (s *HTTPServer) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}
	return s.server.Serve(newLimitedTCPListener(listener.(*net.TCPListener), s.maxConnections))
}

func (s *HTTPServer) Close() error {
	return s.server.Close()
}
