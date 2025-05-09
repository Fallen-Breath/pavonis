package common

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/time/rate"
	"net"
	"strings"
	"time"
)

type RateLimiterFactory func() utils.TransportRateLimiter

type ClientData struct {
	TrafficRateLimiter utils.TransportRateLimiter
	RequestRateLimiter utils.TransportRateLimiter
}

type ClientDataCache struct {
	cfg   *config.Config
	cache *lru.LRU[string, *ClientData]
}

func NewClientDataCache(cfg *config.Config) *ClientDataCache {
	return &ClientDataCache{
		cfg:   cfg,
		cache: lru.NewLRU[string, *ClientData](10240, nil, 3*time.Hour),
	}
}

func ipToKey(ip net.IP) string {
	if ip.To4() != nil {
		return "ipv4$" + ip.String()
	}

	// IPv6
	subnet := ip[:8]
	var hexParts []string
	for _, b := range subnet {
		hexParts = append(hexParts, fmt.Sprintf("%02x", b))
	}
	return "ipv6$" + strings.Join(hexParts, ":")
}

func (c *ClientDataCache) GetData(clientIp string) *ClientData {
	var key string
	if ip := net.ParseIP(clientIp); ip != nil {
		key = ipToKey(ip)
	} else {
		key = "raw$" + clientIp
	}
	value, ok := c.cache.Get(key)
	if ok {
		return value
	}

	limiter := c.newClientData()
	c.cache.Add(key, limiter)
	return limiter
}

func (c *ClientDataCache) newClientData() *ClientData {
	// TODO: configurable
	// 10MiB/s avg, 100MiB burst, 125MiB/s max
	trafficRateLimiter := utils.NewMultiRateLimiter(
		rate.NewLimiter(rate.Limit(10*1048576), 100*1048576),
		rate.NewLimiter(rate.Limit(125*1048576), 125*1048576),
	)

	const qps = 30
	const qpm = 100
	requestRateLimiter := utils.NewMultiRateLimiter(
		rate.NewLimiter(rate.Limit(qps), qps),
		rate.NewLimiter(rate.Limit(qpm/60.0), qpm),
	)

	return &ClientData{
		TrafficRateLimiter: trafficRateLimiter,
		RequestRateLimiter: requestRateLimiter,
	}
}

func (c *ClientDataCache) Clear() {
	c.cache.Purge()
}
