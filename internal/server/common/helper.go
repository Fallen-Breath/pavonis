package common

import (
	"errors"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type RequestHelper struct {
	requestHelperCommon
	ipPoolStrategy config.IpPoolStrategy
}

func (h *RequestHelper) GetTransportForClientIp(clientIp string) (http.RoundTripper, utils.TransportReleaser, error) {
	// concurrency control
	clientData := h.clientDataCache.GetData(clientIp)
	if !clientData.RequestRateLimiter.Allow() {
		return nil, nil, NewHttpError(http.StatusTooManyRequests, "Too many requests")
	}

	var localAddr net.IP
	switch h.ipPoolStrategy {
	case config.IpPoolStrategyNone:
		localAddr = nil
	case config.IpPoolStrategyRandom:
		localAddr = h.ipPool.GetRandomly()
	case config.IpPoolStrategyIpHash:
		localAddr = h.ipPool.GetByKey(clientIp)
	default:
		panic(fmt.Sprintf("Unknown IP strategy: %s", h.ipPoolStrategy))
	}
	log.Debugf("Transport IP for client %s is %s", clientIp, localAddr)

	transport, transportReleaser := h.transportCache.GetTransport(localAddr)
	trafficLimiter := utils.NewMultiRateLimiter(clientData.TrafficRateLimiter)
	if h.globalTrafficLimiter != nil {
		trafficLimiter.AddLimiter(h.globalTrafficLimiter)
	}
	return NewTrafficRateLimitedTransport(transport, trafficLimiter), transportReleaser, nil
}

var headersToRemove = []string{
	"host", // overridden in httputil.ProxyRequest.Out.Host

	// reversed proxy stuffs
	"via", // caddy v2.10.0 adds this
	"x-forwarded-for",
	"x-forwarded-proto",
	"x-forwarded-host",

	// reversed proxy stuffs (cloudflare)
	"cdn-loop",
	"cf-connecting-ip",
	"cf-connecting-ipv6",
	"cf-ew-via",
	"cf-ipcountry",
	"cf-pseudo-ipv4",
	"cf-ray",
	"cf-visitor",
	"cf-warp-tag-id",
}

func (h *RequestHelper) CreateRewrite(destination *url.URL) func(pr *httputil.ProxyRequest) {
	return func(pr *httputil.ProxyRequest) {
		pr.Out.URL = destination
		pr.Out.Host = destination.Host

		for _, header := range headersToRemove {
			pr.Out.Header.Del(header)
		}

		log.Debugf("Downstream request: %+v", pr.Out)
	}
}

func errorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	var httpErr *HttpError
	if errors.As(err, &httpErr) {
		http.Error(w, httpErr.Message, httpErr.Status)
		return
	}
	log.Errorf("http: proxy error: %v", err)
	w.WriteHeader(http.StatusBadGateway)
}

func (h *RequestHelper) RunReverseProxy(w http.ResponseWriter, r *http.Request, destination *url.URL, responseModifier func(resp *http.Response) error) {
	transport, transportReleaser, err := h.GetTransportForClientIp(utils.GetRequestClientIp(r))
	if err != nil {
		errorHandler(w, r, err)
		return
	}
	defer transportReleaser()

	logrusLogger, logrusLoggerCloser := utils.CreateLogrusStdLogger(log.ErrorLevel)
	defer logrusLoggerCloser()

	transport = NewRedirectFollowingTransport(transport, 10)
	transport = NewTimeLimitedTransport(transport, 1*time.Hour)

	proxy := httputil.ReverseProxy{
		Transport:      transport,
		Rewrite:        h.CreateRewrite(destination),
		ModifyResponse: responseModifier,
		ErrorLog:       logrusLogger,
		ErrorHandler:   errorHandler,
	}
	proxy.ServeHTTP(w, r)
}
