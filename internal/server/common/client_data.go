package common

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	expirelru "github.com/hashicorp/golang-lru/v2/expirable"
	"net"
	"strings"
	"time"
)

type RateLimiterFactory func() utils.RateLimiter

type ClientData struct {
	TrafficRateLimiter utils.RateLimiter
	RequestRateLimiter utils.RateLimiter
}

type ClientDataCache struct {
	cfg   *config.Config
	cache *expirelru.LRU[string, *ClientData]
}

func NewClientDataCache(cfg *config.Config) *ClientDataCache {
	return &ClientDataCache{
		cfg:   cfg,
		cache: expirelru.NewLRU[string, *ClientData](10240, nil, 2*time.Hour),
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
	rlc := c.cfg.ResourceLimit

	trafficRateLimiter := utils.CreateTrafficRateLimiter(rlc.TrafficAvgMibps, rlc.TrafficBurstMib, rlc.TrafficMaxMibps)
	requestRateLimiter := utils.CreateRequestRateLimiter(rlc.RequestPerSecond, rlc.RequestPerMinute, rlc.RequestPerHour)

	return &ClientData{
		TrafficRateLimiter: trafficRateLimiter,
		RequestRateLimiter: requestRateLimiter,
	}
}

func (c *ClientDataCache) Clear() {
	c.cache.Purge()
}
