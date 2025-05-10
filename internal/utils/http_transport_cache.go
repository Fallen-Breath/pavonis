package utils

import (
	"context"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type HttpTransportCache struct {
	maxSize      int
	idleTTL      time.Duration
	requestProxy *url.URL

	cache        *sync.Map
	mu           sync.Mutex
	shutdown     bool
	houseKeeping *time.Ticker
}

type transportEntry struct {
	localAddress net.IP
	transport    *http.Transport
	lastUsed     time.Time
	inUseCount   int
}

func NewHttpTransportCache(maxSize int, idleTTL time.Duration, requestProxy *url.URL) *HttpTransportCache {
	cache := &HttpTransportCache{
		maxSize:      maxSize,
		idleTTL:      idleTTL,
		requestProxy: requestProxy,
		cache:        &sync.Map{},
		houseKeeping: time.NewTicker(1 * time.Second),
	}
	go cache.houseKeepingRoutine()
	return cache
}

type TransportReleaser func()

func (c *HttpTransportCache) GetTransport(localAddress net.IP) (*http.Transport, TransportReleaser) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.shutdown {
		log.Error("HttpTransportCache has been shutdown")
		return nil, nil
	}

	var key string
	if localAddress == nil {
		key = "<nil>"
	} else {
		key = localAddress.String()
	}
	entry, ok := c.cache.Load(key)
	if !ok {
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var localAddr net.Addr
				if localAddress != nil {
					localAddr = &net.TCPAddr{IP: localAddress}
				} else {
					localAddr = nil
				}
				dialer := &net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
					LocalAddr: localAddr,
				}
				return dialer.DialContext(ctx, network, addr)
			},
			Proxy: func(req *http.Request) (*url.URL, error) {
				return c.requestProxy, nil
			},
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConns:          32,
			MaxIdleConnsPerHost:   32,
			MaxConnsPerHost:       4,
		}
		entry = &transportEntry{
			localAddress: localAddress,
			transport:    transport,
			lastUsed:     time.Now(),
			inUseCount:   0,
		}
		c.cache.Store(key, entry)
		log.Debugf("Created new transport for local address: %s", localAddress)
	}

	entry.(*transportEntry).inUseCount++
	entry.(*transportEntry).lastUsed = time.Now()

	releaseFunc := func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		entry.(*transportEntry).inUseCount--
		entry.(*transportEntry).lastUsed = time.Now()
	}

	return entry.(*transportEntry).transport, releaseFunc
}

func (c *HttpTransportCache) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdown = true
	c.houseKeeping.Stop()

	c.cache.Range(func(key, value interface{}) bool {
		entry := value.(*transportEntry)
		log.Debugf("Shutting down transport for %s is no longer in use", entry.localAddress)
		return true
	})
	c.cache = &sync.Map{}
}

func (c *HttpTransportCache) houseKeepingRoutine() {
	doHousekeeping := func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.shutdown {
			return
		}

		now := time.Now()
		var toRemove []string
		c.cache.Range(func(key, value interface{}) bool {
			entry := value.(*transportEntry)
			if entry.inUseCount == 0 && now.Sub(entry.lastUsed) > c.idleTTL {
				toRemove = append(toRemove, key.(string))
			}
			return true
		})
		for _, key := range toRemove {
			c.cache.Delete(key)
			log.Debugf("Removed idle transport for local address: %s", key)
		}
	}

	for range c.houseKeeping.C {
		doHousekeeping()
	}
}
