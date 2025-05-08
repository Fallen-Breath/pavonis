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

type HttpError struct {
	Status  int
	Message string
}

var _ error = &HttpError{}

func (e *HttpError) Error() string {
	return e.Message
}

type RequestHelperFactory struct {
	ipPool         *utils.IpPool
	transportCache *utils.HttpTransportCache
	cfg            *config.IpPoolConfig
}

func NewRequestHelperFactory(cfg *config.Config) *RequestHelperFactory {
	var ipPool *utils.IpPool = nil
	if cfg.IpPool.Enabled {
		var err error
		ipPool, err = utils.NewIpPool(cfg.IpPool.Subnets)
		if err != nil {
			log.Fatalf("Failed to create ip pool: %v", err)
		}
	}

	return &RequestHelperFactory{
		ipPool:         ipPool,
		transportCache: utils.NewHttpTransportCache(256, 30*time.Second),
		cfg:            cfg.IpPool,
	}
}

type RequestHelper struct {
	ipPool         *utils.IpPool
	transportCache *utils.HttpTransportCache
	ipPoolStrategy config.IpPoolStrategy
}

func (f *RequestHelperFactory) NewRequestHelper(siteIpPoolStrategy *config.IpPoolStrategy) *RequestHelper {
	ipPoolStrategy := config.IpPoolStrategyNone
	if f.cfg.Enabled {
		ipPoolStrategy = f.cfg.DefaultStrategy
		if siteIpPoolStrategy != nil {
			ipPoolStrategy = *siteIpPoolStrategy
		}
	}

	return &RequestHelper{
		ipPool:         f.ipPool,
		transportCache: f.transportCache,
		ipPoolStrategy: ipPoolStrategy,
	}
}

func (f *RequestHelperFactory) Shutdown() {
	f.transportCache.Shutdown()
}

func (h *RequestHelper) GetTransportForClient(r *http.Request) (*http.Transport, utils.TransportReleaser) {
	clientIp := utils.GetRequestClientIp(r)
	return h.GetTransportForIp(clientIp)
}

func (h *RequestHelper) GetTransportForIp(clientIp string) (*http.Transport, utils.TransportReleaser) {
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
	return h.transportCache.GetTransport(localAddr)
}

var headersToRemove = []string{
	"host", // overriden in httputil.ProxyRequest.Out.Host

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
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			var httpErr *HttpError
			if errors.As(err, &httpErr) {
				http.Error(w, httpErr.Message, httpErr.Status)
				return
			}
			log.Errorf("http: proxy error: %v", err)
			w.WriteHeader(http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}
