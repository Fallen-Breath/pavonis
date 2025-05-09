package common

import (
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"time"
)

type requestHelperCommon struct {
	ipPool               *utils.IpPool
	transportCache       *utils.HttpTransportCache
	clientDataCache      *ClientDataCache
	globalTrafficLimiter *rate.Limiter
}

type RequestHelperFactory struct {
	requestHelperCommon
	cfg *config.IpPoolConfig
}

func NewRequestHelperFactory(cfg *config.Config) *RequestHelperFactory {
	var ipPool *utils.IpPool = nil
	if cfg.IpPool.Enabled {
		if len(cfg.IpPool.Subnets) == 0 {
			log.Fatalf("IpPool has no subnet")
		}
		var err error
		ipPool, err = utils.NewIpPool(cfg.IpPool.Subnets)
		if err != nil {
			log.Fatalf("Failed to create ip pool: %v", err)
		}
	}

	clientDataCache := NewClientDataCache(cfg)

	return &RequestHelperFactory{
		requestHelperCommon: requestHelperCommon{
			ipPool:               ipPool,
			transportCache:       utils.NewHttpTransportCache(1024, 60*time.Second),
			clientDataCache:      clientDataCache,
			globalTrafficLimiter: nil, // TODO
		},
		cfg: cfg.IpPool,
	}
}

func (f *RequestHelperFactory) NewRequestHelper(siteIpPoolStrategy *config.IpPoolStrategy) *RequestHelper {
	ipPoolStrategy := config.IpPoolStrategyNone
	if f.cfg.Enabled {
		ipPoolStrategy = *f.cfg.DefaultStrategy
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
