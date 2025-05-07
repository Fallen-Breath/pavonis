package common

import (
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
	ipPool         *utils.IpPool
	transportCache *utils.HttpTransportCache
	cfg            *config.IpPoolConfig
}

func NewRequestHelper(cfg *config.Config) *RequestHelper {
	var ipPool *utils.IpPool = nil
	if cfg.IpPool.Strategy != config.IpPoolStrategyNone {
		var err error
		ipPool, err = utils.NewIpPool(cfg.IpPool.Subnets)
		if err != nil {
			log.Fatalf("Failed to create ip pool: %v", err)
		}
	}

	return &RequestHelper{
		ipPool:         ipPool,
		transportCache: utils.NewHttpTransportCache(256, 30*time.Second),
		cfg:            cfg.IpPool,
	}
}

func (h *RequestHelper) Shutdown() {
	h.transportCache.Shutdown()
}

func (h *RequestHelper) GetTransportForClient(r *http.Request) (*http.Transport, utils.TransportReleaser) {
	clientIp := utils.GetRequestClientIp(r)
	return h.GetTransportForIp(clientIp)
}

func (h *RequestHelper) GetTransportForIp(clientIp string) (*http.Transport, utils.TransportReleaser) {
	var localAddr net.IP
	switch h.cfg.Strategy {
	case config.IpPoolStrategyNone:
		localAddr = nil
	case config.IpPoolStrategyRandom:
		localAddr = h.ipPool.GetRandomly()
	case config.IpPoolStrategyIpHash:
		localAddr = h.ipPool.GetByKey(clientIp)
	default:
		panic(fmt.Sprintf("Unknown IP strategy: %s", h.cfg.Strategy))
	}
	log.Infof("Transport IP for client %s is %s", clientIp, localAddr)
	return h.transportCache.GetTransport(localAddr)
}

var headersToRemove = []string{
	"host",
	"x-forwarded-for",
	"x-forwarded-proto",
	"x-forwarded-host",
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

func (h *RequestHelper) FixDownstreamRequest(r *http.Request) {
	for _, header := range headersToRemove {
		r.Header.Del(header)
	}
}

func (h *RequestHelper) CreateRewrite(destination *url.URL) func(pr *httputil.ProxyRequest) {
	return func(pr *httputil.ProxyRequest) {
		pr.Out.URL = destination
		pr.Out.Host = destination.Host
		h.FixDownstreamRequest(pr.Out)
	}
}

func (h *RequestHelper) RunReverseProxy(w http.ResponseWriter, r *http.Request, destination *url.URL, responseModifier func(resp *http.Response) error) {
	transport, transportReleaser := h.GetTransportForClient(r)
	defer transportReleaser()

	logrusLogger, logrusLoggerCloser := utils.CreateLogrusStdLogger(log.ErrorLevel)
	defer logrusLoggerCloser()

	proxy := httputil.ReverseProxy{
		Transport: &RedirectFollowingTransport{
			Transport:    transport,
			MaxRedirects: 10,
		},
		Rewrite:        h.CreateRewrite(destination),
		ModifyResponse: responseModifier,
		ErrorLog:       logrusLogger,
	}
	proxy.ServeHTTP(w, r)
}
