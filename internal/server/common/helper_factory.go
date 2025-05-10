package common

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"net/url"
	"time"
)

type requestHelperCommon struct {
	cfg                  *config.Config
	ipPool               *utils.IpPool
	transportCache       *utils.HttpTransportCache
	clientDataCache      *ClientDataCache
	globalTrafficLimiter *rate.Limiter
}

type RequestHelperFactory struct {
	requestHelperCommon
}

func NewRequestHelperFactory(cfg *config.Config) (*RequestHelperFactory, error) {
	ipPoolCfg := cfg.Request.IpPool

	var ipPool *utils.IpPool = nil
	if ipPoolCfg.Enabled {
		if len(ipPoolCfg.Subnets) == 0 {
			log.Fatalf("IpPool has no subnet")
		}
		var err error
		ipPool, err = utils.NewIpPool(ipPoolCfg.Subnets)
		if err != nil {
			log.Fatalf("Failed to create ip pool: %v", err)
		}
	}

	clientDataCache := NewClientDataCache(cfg)
	var requestProxy *url.URL = nil
	if cfg.Request.Proxy != "" {
		var err error
		requestProxy, err = url.Parse(cfg.Request.Proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy url %q: %v", cfg.Request.Proxy, err)
		}
	}

	return &RequestHelperFactory{
		requestHelperCommon: requestHelperCommon{
			cfg:                  cfg,
			ipPool:               ipPool,
			transportCache:       utils.NewHttpTransportCache(1024, 60*time.Second, requestProxy),
			clientDataCache:      clientDataCache,
			globalTrafficLimiter: nil, // TODO
		},
	}, nil
}

func (f *RequestHelperFactory) NewRequestHelper(siteIpPoolStrategy *config.IpPoolStrategy) *RequestHelper {
	ipPoolCfg := f.cfg.Request.IpPool

	ipPoolStrategy := config.IpPoolStrategyNone
	if ipPoolCfg.Enabled {
		ipPoolStrategy = *ipPoolCfg.DefaultStrategy
		if siteIpPoolStrategy != nil {
			ipPoolStrategy = *siteIpPoolStrategy
		}
	}

	return &RequestHelper{
		requestHelperCommon: f.requestHelperCommon,
		ipPoolStrategy:      ipPoolStrategy,
	}
}

func (f *RequestHelperFactory) Shutdown() {
	f.transportCache.Shutdown()
	f.clientDataCache.Clear()
}
