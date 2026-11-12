package server

import (
	gocontext "context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/felixge/httpsnoop"
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

	for sideIdx, siteCfg := range cfg.Sites {
		siteInfo := handler.NewSiteInfo(siteCfg.Id, siteCfg)
		helper := helperFactory.NewRequestHelper(siteCfg.Id, siteCfg.IpPoolStrategy)

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
	targetHandlerId := ""
	handlerNamePrefix := ""
	if targetHandler != nil {
		targetHandlerId = targetHandler.Info().Id
		handlerNamePrefix = targetHandlerId + ":"
	}
	ctx.LogPrefix = fmt.Sprintf("(%s%s) ", handlerNamePrefix, ctx.RequestId)

	// start logging
	logLine := ctx.LogPrefix + fmt.Sprintf("%s - %s %s", ctx.ClientAddr, r.Method, r.URL.Path)
	log.
		WithField("Host", ctx.Host).
		WithField("UA", sll(r.UserAgent(), 24)).
		Debug(logLine)

	// adjust request context
	rContext := r.Context()
	rContext = gocontext.WithValue(rContext, context.Key, ctx)
	reqContext, cancel := gocontext.WithTimeout(rContext, *s.cfg.ResourceLimit.RequestTimeout) // total timeout
	defer cancel()
	r = r.WithContext(reqContext)
	if r.Body != nil && r.Body != http.NoBody {
		deadline, _ := reqContext.Deadline()
		// Do not clear this deadline on return: net/http may still drain the body.
		// net/http resets it before processing the next request.
		if err := http.NewResponseController(w).SetReadDeadline(deadline); err != nil {
			log.Warnf("%sFailed to set request body read deadline: %v", ctx.LogPrefix, err)
		}
	}

	// http metrics
	hm := httpsnoop.Metrics{Code: http.StatusOK}
	var body *countingRequestBody
	if r.Body != nil && r.Body != http.NoBody {
		body = &countingRequestBody{ReadCloser: r.Body}
		r.Body = body
	}
	requestsInFlight := metricRequestsInFlight.WithLabelValues(targetHandlerId, r.Method)
	requestsInFlight.Inc()
	defer requestsInFlight.Dec()

	defer func() {
		// end logging
		panicErr := recover()
		if panicErr == http.ErrAbortHandler {
			logLine += " (aborted)"
		} else if panicErr != nil {
			logLine += " (panic)"
		}

		costSec := time.Since(ctx.StartTime).Seconds()
		log.
			WithField("Cost", fmt.Sprintf("%.3fs", costSec)).
			WithField("Status", fmt.Sprintf("%d %s", hm.Code, http.StatusText(hm.Code))).
			Info(logLine)

		code := strconv.Itoa(hm.Code)
		metricRequestServed.WithLabelValues(targetHandlerId, r.Method, code).Inc()
		metricRequestDuration.WithLabelValues(targetHandlerId, r.Method, code).Observe(costSec)
		var requestBytes int64
		if body != nil {
			requestBytes = body.bytesRead.Load()
		}
		metricRequestSize.WithLabelValues(targetHandlerId, r.Method, code).Observe(float64(requestBytes))
		metricResponseSize.WithLabelValues(targetHandlerId, r.Method, code).Observe(float64(hm.Written))
		if ctx.RequestRateLimited {
			metricRateLimitRejections.WithLabelValues(targetHandlerId).Inc()
		}

		if panicErr != nil {
			panic(panicErr)
		}
	}()

	// process the request
	hm.CaptureMetrics(w, func(w http.ResponseWriter) {
		if isWebSocketRequest(r) {
			http.Error(w, "WebSocket connections are not allowed", http.StatusForbidden)
			return
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
		if _, ok := utils.CutPathPrefix(path, candidateHandler.Info().PathPrefix); ok {
			return candidateHandler
		}
	}
	return nil
}

func (s *PavonisServer) CreateHttpServer() *http.Server {
	return &http.Server{
		Addr:    *s.cfg.Server.Listen,
		Handler: s,
	}
}
