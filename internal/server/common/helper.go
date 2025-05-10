package common

import (
	gocontext "context"
	"errors"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
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

func adjustHeader(header http.Header, cfg *config.HeaderModificationConfig) {
	for key, value := range *cfg.Modify {
		header.Set(key, value)
	}
	for _, key := range *cfg.Delete {
		header.Del(key)
	}
}

func (h *RequestHelper) createRequestModifier(destination *url.URL) func(pr *httputil.ProxyRequest) {
	return func(pr *httputil.ProxyRequest) {
		pr.Out.URL = destination
		pr.Out.Host = destination.Host

		adjustHeader(pr.Out.Header, h.cfg.Request.Header)
		pr.Out.Header.Del("host")

		if log.IsLevelEnabled(log.DebugLevel) {
			log.Debugf("Downstream request: %+v", utils.MaskRequestForLogging(pr.Out))
		}
	}
}

type responseModifier func(resp *http.Response) error

func (h *RequestHelper) createResponseModifier(bizModifier responseModifier) responseModifier {
	return func(resp *http.Response) error {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.Debugf("Downstream raw response: %+v", utils.MaskResponseForLogging(resp))
		}

		adjustHeader(resp.Header, h.cfg.Response.Header)

		if bizModifier != nil {
			return bizModifier(resp)
		}
		return nil
	}
}

func errorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	if errors.Is(err, gocontext.DeadlineExceeded) {
		http.Error(w, "request time limit exceeded", http.StatusGatewayTimeout)
		return
	}

	var httpErr *HttpError
	if errors.As(err, &httpErr) {
		http.Error(w, httpErr.Message, httpErr.Status)
		return
	}
	log.Errorf("http: proxy error: %v", err)
	w.WriteHeader(http.StatusBadGateway)
}

func (h *RequestHelper) RunReverseProxy(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request, destination *url.URL, responseModifier responseModifier) {
	transport, transportReleaser, err := h.GetTransportForClientIp(ctx.ClientAddr)
	if err != nil {
		errorHandler(w, r, err)
		return
	}
	defer transportReleaser()

	logrusLogger, logrusLoggerCloser := utils.CreateLogrusStdLogger(log.ErrorLevel)
	defer logrusLoggerCloser()

	proxy := httputil.ReverseProxy{
		Transport:      NewRedirectFollowingTransport(transport, 10),
		Rewrite:        h.createRequestModifier(destination),
		ModifyResponse: h.createResponseModifier(responseModifier),
		ErrorLog:       logrusLogger,
		ErrorHandler:   errorHandler,
	}
	proxy.ServeHTTP(w, r)
}
