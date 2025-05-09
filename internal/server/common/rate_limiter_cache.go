package common

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"net"
	"strings"
	"time"
)

type RateLimiterFactory func() utils.TransportRateLimiter

type RateLimiterCache struct {
	cache          *lru.LRU[string, utils.TransportRateLimiter]
	limiterFactory RateLimiterFactory
}

func NewRateLimiterCache(cacheSize int, factory RateLimiterFactory) *RateLimiterCache {
	return &RateLimiterCache{
		cache:          lru.NewLRU[string, utils.TransportRateLimiter](cacheSize, nil, 6*time.Hour),
		limiterFactory: factory,
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

func (c *RateLimiterCache) GetLimiter(clientIp string) utils.TransportRateLimiter {
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

	limiter := c.limiterFactory()
	c.cache.Add(key, limiter)
	return limiter
}

func (c *RateLimiterCache) Clear() {
	c.cache.Purge()
}
