package common

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"net/url"
	"time"
)

type requestHelperCommon struct {
	cfg            *config.Config
	ipPool         *utils.IpPool
	transportCache *utils.HttpTransportCache
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

	proxyOpts, err := newRequestProxyOptions(cfg.Request.Proxy)
	if err != nil {
		return nil, err
	}

	return &RequestHelperFactory{
		requestHelperCommon: requestHelperCommon{
			cfg:            cfg,
			ipPool:         ipPool,
			transportCache: utils.NewHttpTransportCache(1024, 60*time.Second, proxyOpts),
		},
	}, nil
}

func newRequestProxyOptions(proxyCfg *config.RequestProxyConfig) (utils.RequestProxyOptions, error) {
	opts := utils.RequestProxyOptions{
		DirectFallback: proxyCfg.DirectFallback,
		ConnectTimeout: *proxyCfg.ConnectTimeout,
		Block: utils.RequestProxyBlockOptions{
			FailureThreshold: int(*proxyCfg.Block.FailureThreshold),
			Cooldown:         *proxyCfg.Block.Cooldown,
			MaxCooldown:      *proxyCfg.Block.MaxCooldown,
		},
	}
	for idx, server := range proxyCfg.Servers {
		proxyUrl, err := url.Parse(server.Url)
		if err != nil {
			return opts, fmt.Errorf("failed to parse proxy url #%d: %v", idx, err)
		}
		var version byte
		switch server.ProxyProtocol {
		case config.ProxyProtocolV1:
			version = 1
		case config.ProxyProtocolV2:
			version = 2
		}
		opts.Servers = append(opts.Servers, utils.RequestProxyServerOptions{
			Id:                   server.Id,
			Url:                  proxyUrl,
			ProxyProtocolVersion: version,
			Weight:               *server.Weight,
		})
	}
	switch *proxyCfg.Strategy {
	case config.RequestProxyStrategyByOrder:
		opts.Strategy = utils.RequestProxyStrategyByOrder
	case config.RequestProxyStrategyRandom:
		opts.Strategy = utils.RequestProxyStrategyRandom
	case config.RequestProxyStrategyIpHash:
		opts.Strategy = utils.RequestProxyStrategyIpHash
	default:
		return opts, fmt.Errorf("unknown request proxy strategy %q", *proxyCfg.Strategy)
	}
	return opts, nil
}

func (f *RequestHelperFactory) NewRequestHelper(siteId string, siteIpPoolStrategy *config.IpPoolStrategy) *RequestHelper {
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
		siteId:              siteId,
		ipPoolStrategy:      ipPoolStrategy,
	}
}

func (f *RequestHelperFactory) ReadRequestProxyBlockStates() map[string]bool {
	return f.transportCache.ReadRequestProxyBlockStates()
}

func (f *RequestHelperFactory) Shutdown() {
	f.transportCache.Shutdown()
}
