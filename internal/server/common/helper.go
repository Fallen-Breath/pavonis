package common

import (
	gocontext "context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
)

type RequestHelper struct {
	requestHelperCommon
	siteId              string
	ipPoolStrategy      config.IpPoolStrategy
	checkDirectDialAddr func(netip.AddrPort) error
	transportOverride   http.RoundTripper
}

// The returned helper shares the connection pool with h, so h must not have sent any request
func (h *RequestHelper) WithCheckDirectDialAddr(check func(netip.AddrPort) error) *RequestHelper {
	newHelper := *h
	newHelper.checkDirectDialAddr = check
	return &newHelper
}

// WithTransportOverride makes the returned helper send all upstream requests via the given transport.
// It's for tests only
func (h *RequestHelper) WithTransportOverride(transport http.RoundTripper) *RequestHelper {
	newHelper := *h
	newHelper.transportOverride = transport
	return &newHelper
}

func (h *RequestHelper) getTransportForClientIp(ctx *context.RequestContext) (http.RoundTripper, utils.TransportReleaser, error) {
	clientIp := ctx.ClientAddr

	// concurrency control
	clientData := h.clientDataCache.GetOrCreateAndRefresh(clientIp)
	if !clientData.RequestRateLimiter.Allow() {
		ctx.RequestRateLimited = true
		return nil, nil, NewHttpError(http.StatusTooManyRequests, "Too many requests")
	}

	var localAddr net.IP
	switch h.ipPoolStrategy {
	case config.IpPoolStrategyNone:
		localAddr = nil
	case config.IpPoolStrategyRandom:
		localAddr = h.ipPool.GetRandomly()
	case config.IpPoolStrategyIpHash:
		localAddr = h.ipPool.GetByKey(utils.GetBucketForIpString(clientIp))
	default:
		panic(fmt.Sprintf("Unknown IP strategy: %s", h.ipPoolStrategy))
	}
	log.Debugf("%sTransport IP for client %s is %s", ctx.LogPrefix, clientIp, localAddr)

	var proxyProtocolAddrs *utils.ProxyProtocolAddrs
	if h.transportCache.NeedsProxyProtocolAddrs() {
		clientAddr, err := netip.ParseAddr(ctx.ClientAddr)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid client IP for PROXY protocol: %w", err)
		}
		proxyProtocolAddrs = &utils.ProxyProtocolAddrs{
			Source:      netip.AddrPortFrom(clientAddr, ctx.ClientPort),
			Destination: ctx.ClientLocalAddr,
		}
	}
	var transport http.RoundTripper
	var transportReleaser utils.TransportReleaser
	if h.transportOverride != nil {
		transport, transportReleaser = h.transportOverride, func() {}
	} else {
		transport, transportReleaser = h.transportCache.GetTransport(utils.TransportOptions{
			PoolKey:             h.siteId,
			LocalAddress:        localAddr,
			ProxyProtocolAddrs:  proxyProtocolAddrs,
			CheckDirectDialAddr: h.checkDirectDialAddr,
		})
	}
	trafficLimiter := utils.NewMultiRateLimiter(clientData.TrafficRateLimiter, h.globalTrafficLimiter)
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

func (h *RequestHelper) createRequestModifier(ctx *context.RequestContext, destination *url.URL) func(pr *httputil.ProxyRequest) {
	return func(pr *httputil.ProxyRequest) {
		pr.Out.URL = destination
		pr.Out.Host = destination.Host

		adjustHeader(pr.Out.Header, h.cfg.Request.Header)
		pr.Out.Header.Del("host")

		if log.IsLevelEnabled(log.DebugLevel) {
			log.Debugf("%sUpstream request: %+v", ctx.LogPrefix, utils.MaskRequestForLogging(pr.Out))
		}
	}
}

type ResponseModifier func(lastReq *http.Request, resp *http.Response) error

func (h *RequestHelper) createResponseModifier(ctx *context.RequestContext, bizModifier ResponseModifier, reqHistory *[]*http.Request) func(resp *http.Response) error {
	return func(resp *http.Response) error {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.Debugf("%sUpstream raw response: %+v", ctx.LogPrefix, utils.MaskResponseForLogging(resp))
		}

		adjustHeader(resp.Header, h.cfg.Response.Header)

		if len(*reqHistory) == 0 {
			panic(fmt.Sprintf("%sreqHistory is empty", ctx.LogPrefix))
		}
		if bizModifier != nil {
			lastReq := (*reqHistory)[len(*reqHistory)-1]
			return bizModifier(lastReq, resp)
		}
		return nil
	}
}

func (h *RequestHelper) createErrorHandler(ctx *context.RequestContext) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		// A body read timeout can yield context.Canceled before the context timer fires.
		deadline, hasDeadline := r.Context().Deadline()
		if errors.Is(err, gocontext.DeadlineExceeded) || (hasDeadline && !time.Now().Before(deadline)) {
			http.Error(w, "request timed out", http.StatusGatewayTimeout)
			return
		}

		var httpErr *HttpError
		if errors.As(err, &httpErr) {
			http.Error(w, httpErr.Message, httpErr.Status)
			return
		}
		log.Errorf("%sproxy error: %v", ctx.LogPrefix, err)
		w.WriteHeader(http.StatusBadGateway)
	}
}

// Response processing order:
// 1. Prepare request
//   a) createRequestModifier()
// 2. Following redirects (possible multiple times)
//   a) call RequestModifier() before each upstream request
//   b) call RedirectHandler()
//   c) optionally rewrite the "Location" header (in RedirectFollowingTransport)
// 3. Process the final response:
//   a) call adjustHeader()
//   b) call ResponseModifier()

func (h *RequestHelper) RunReverseProxy(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request, destination *url.URL, opts ...ReverseProxyOption) {
	rrConfig := &RunReverseProxyConfig{
		ResponseModifier: func(lastReq *http.Request, resp *http.Response) error {
			return nil
		},
		RedirectHandler: func(resp *http.Response) *RedirectResult {
			return &RedirectResult{Decision: RedirectDecisionFollow}
		},
	}
	for _, opt := range opts {
		opt(rrConfig)
	}

	// The errors returned by the size limited readers are not guaranteed to be passed through
	// the transport or the ReverseProxy, so check the reader states directly
	var limitedReqBody, limitedRespBody *TrafficSizeLimitedReadCloser
	if rrConfig.RequestSizeLimit > 0 {
		if r.ContentLength > rrConfig.RequestSizeLimit {
			http.Error(w, "Request too large", http.StatusForbidden)
			return
		}
		if r.Body != nil && r.Body != http.NoBody {
			limitedReqBody = NewTrafficSizeLimitedReadCloser(r.Body, rrConfig.RequestSizeLimit)
			r.Body = limitedReqBody
		}
	}
	if rrConfig.ResponseSizeLimit > 0 {
		bizModifier := rrConfig.ResponseModifier
		rrConfig.ResponseModifier = func(lastReq *http.Request, resp *http.Response) error {
			if resp.ContentLength > rrConfig.ResponseSizeLimit {
				return NewHttpError(http.StatusForbidden, "Response too large")
			}
			if resp.ContentLength < 0 {
				limitedRespBody = NewTrafficSizeLimitedReadCloser(resp.Body, rrConfig.ResponseSizeLimit)
				resp.Body = limitedRespBody
			}
			return bizModifier(lastReq, resp)
		}
	}

	baseErrorHandler := h.createErrorHandler(ctx)
	errorHandlerCalled := false
	errorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		errorHandlerCalled = true
		if limitedReqBody != nil && limitedReqBody.Exceeded() {
			http.Error(w, "Request too large", http.StatusForbidden)
			return
		}
		if limitedRespBody != nil && limitedRespBody.Exceeded() {
			http.Error(w, "Response too large", http.StatusForbidden)
			return
		}
		baseErrorHandler(w, r, err)
	}

	transport, transportReleaser, err := h.getTransportForClientIp(ctx)
	if err != nil {
		errorHandler(w, r, err)
		return
	}
	defer transportReleaser()

	logrusLogger, logrusLoggerCloser := utils.CreateLogrusStdLogger(log.ErrorLevel)
	defer logrusLoggerCloser()

	requestHistory := new([]*http.Request)
	requestModifier := h.createRequestModifier(ctx, destination)
	responseModifier := h.createResponseModifier(ctx, rrConfig.ResponseModifier, requestHistory)

	transport = NewRedirectFollowingTransport(ctx, transport, *h.cfg.Response.MaxRedirect, rrConfig.RedirectHandler, func(req *http.Request) {
		if rrConfig.RequestModifier != nil {
			rrConfig.RequestModifier(req)
		}
		*requestHistory = append(*requestHistory, req)
	})

	proxy := httputil.ReverseProxy{
		Transport:      transport,
		Rewrite:        requestModifier,
		ModifyResponse: responseModifier,
		ErrorLog:       logrusLogger,
		ErrorHandler:   errorHandler,
	}
	proxy.ServeHTTP(w, r)

	if limitedRespBody != nil && limitedRespBody.Exceeded() && !errorHandlerCalled {
		// The response header has already been sent, aborting is the only way to let the client know
		// that the response is incomplete. See the doc of http.Handler and http.ErrAbortHandler
		panic(http.ErrAbortHandler)
	}
}

// FetchHeader sends a bodyless GET request to the upstream without following redirects,
// and returns the response status and header. The response body is discarded
func (h *RequestHelper) FetchHeader(ctx *context.RequestContext, r *http.Request, destination *url.URL) (int, http.Header, error) {
	transport, transportReleaser, err := h.getTransportForClientIp(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer transportReleaser()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, destination.String(), nil)
	if err != nil {
		return 0, nil, err
	}
	adjustHeader(req.Header, h.cfg.Request.Header)
	if log.IsLevelEnabled(log.DebugLevel) {
		log.Debugf("%sUpstream header-only request: %+v", ctx.LogPrefix, utils.MaskRequestForLogging(req))
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		return 0, nil, err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	_ = resp.Body.Close()
	return resp.StatusCode, resp.Header, nil
}
