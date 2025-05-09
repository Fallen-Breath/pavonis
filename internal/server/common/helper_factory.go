package common

import (
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"time"
)

type requestHelperCommon struct {
	ipPool            *utils.IpPool
	transportCache    *utils.HttpTransportCache
	rateLimiterCache  *RateLimiterCache
	globalRateLimiter *rate.Limiter
}

type RequestHelperFactory struct {
	requestHelperCommon
	cfg *config.IpPoolConfig
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

	// TODO: configurable
	rateLimiterCache := NewRateLimiterCache(10240, func() utils.TransportRateLimiter {
		// 10MiB/s avg, 100MiB burst, 125MiB/s max
		return utils.NewMultiRateLimiter(
			rate.NewLimiter(rate.Limit(10*1048576), 100*1048576),
			rate.NewLimiter(rate.Limit(125*1048576), 125*1048576),
		)
	})

	return &RequestHelperFactory{
		requestHelperCommon: requestHelperCommon{
			ipPool:            ipPool,
			transportCache:    utils.NewHttpTransportCache(1024, 60*time.Second),
			rateLimiterCache:  rateLimiterCache,
			globalRateLimiter: nil, // TODO
		},
		cfg: cfg.IpPool,
	}
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
		requestHelperCommon: f.requestHelperCommon,
		ipPoolStrategy:      ipPoolStrategy,
	}
}

func (f *RequestHelperFactory) Shutdown() {
	f.transportCache.Shutdown()
	f.rateLimiterCache.Clear()
}
