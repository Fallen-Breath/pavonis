package common

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	expirelru "github.com/hashicorp/golang-lru/v2/expirable"
	"math"
	"net"
	"strings"
	"sync"
	"time"
)

type ClientData struct {
	TrafficRateLimiter utils.RateLimiter
	RequestRateLimiter utils.RateLimiter
}

type ClientDataCache struct {
	cfg   *config.Config
	cache *expirelru.LRU[string, *ClientData]
	mu    sync.Mutex
}

func NewClientDataCache(cfg *config.Config) *ClientDataCache {
	rlc := cfg.ResourceLimit
	ttl := *rlc.RequestTimeout
	// Recreating a client must not grant tokens before its buckets refill.
	ttl = max(ttl,
		utils.CalculateRequestRateLimitRefillDuration(rlc.RequestPerSecond, time.Second),
		utils.CalculateRequestRateLimitRefillDuration(rlc.RequestPerMinute, time.Minute),
		utils.CalculateRequestRateLimitRefillDuration(rlc.RequestPerHour, time.Hour),
	)
	// Preserve the cleanup margin without overflowing large request timeouts.
	ttl += min(time.Minute, time.Duration(math.MaxInt64)-ttl)

	return &ClientDataCache{
		cfg:   cfg,
		cache: expirelru.NewLRU[string, *ClientData](10240, nil, ttl),
	}
}

func ipToKey(ip net.IP) string {
	if ip.To4() != nil {
		return "ipv4$" + ip.String()
	}

	// IPv6
	subnet := ip[:8] // take /64
	var hexParts []string
	for _, b := range subnet {
		hexParts = append(hexParts, fmt.Sprintf("%02x", b))
	}
	return "ipv6$" + strings.Join(hexParts, ":")
}

// GetOrCreateAndRefresh returns the client's state and renews its cache TTL.
func (c *ClientDataCache) GetOrCreateAndRefresh(clientIp string) *ClientData {
	var key string
	if ip := net.ParseIP(clientIp); ip != nil {
		key = ipToKey(ip)
	} else {
		key = "raw$" + clientIp
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.cache.Get(key)
	if !ok {
		value = c.newClientData()
	}
	c.cache.Add(key, value)
	return value
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
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Purge()
}
