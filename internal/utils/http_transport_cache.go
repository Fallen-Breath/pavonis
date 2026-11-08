package utils

import (
	"context"
	"errors"
	"fmt"
	proxyproto "github.com/pires/go-proxyproto"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type HttpTransportCache struct {
	maxSize         int
	idleTTL         time.Duration
	proxyCandidates []*proxyCandidate

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

func NewHttpTransportCache(maxSize int, idleTTL time.Duration, proxyOpts RequestProxyOptions) *HttpTransportCache {
	cache := &HttpTransportCache{
		maxSize:         maxSize,
		idleTTL:         idleTTL,
		proxyCandidates: newProxyCandidates(proxyOpts),
		cache:           &sync.Map{},
		houseKeeping:    time.NewTicker(1 * time.Second),
	}
	go cache.houseKeepingRoutine()
	return cache
}

func (c *HttpTransportCache) NeedsProxyProtocolAddrs() bool {
	for _, candidate := range c.proxyCandidates {
		if candidate.proxyProtocolVersion != 0 {
			return true
		}
	}
	return false
}

type ProxyProtocolAddrs struct {
	Source      netip.AddrPort
	Destination netip.AddrPort
}

type TransportReleaser func()

// proxyProtocolAddrs must not be nil if NeedsProxyProtocolAddrs returns true
func (c *HttpTransportCache) GetTransport(localAddress net.IP, proxyProtocolAddrs *ProxyProtocolAddrs) (http.RoundTripper, TransportReleaser) {
	transport := &proxyFailoverTransport{
		cache:              c,
		localAddress:       localAddress,
		proxyProtocolAddrs: proxyProtocolAddrs,
	}
	return transport, transport.release
}

// Only requests without a body are resent via the next candidate,
// since http.Transport closes the request body when RoundTrip fails
type proxyFailoverTransport struct {
	cache              *HttpTransportCache
	localAddress       net.IP
	proxyProtocolAddrs *ProxyProtocolAddrs

	mu        sync.Mutex
	released  bool
	releasers []TransportReleaser
}

var _ http.RoundTripper = &proxyFailoverTransport{}

func (t *proxyFailoverTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	candidates := t.cache.proxyCandidates
	candidate := selectProxyCandidate(candidates, time.Now())
	for {
		transport, err := t.acquireTransport(candidate)
		if err != nil {
			if req.Body != nil {
				_ = req.Body.Close()
			}
			return nil, err
		}

		resp, err := transport.RoundTrip(req)
		if err == nil {
			return resp, nil
		}

		var dialErr *proxyDialError
		if !errors.As(err, &dialErr) || (req.Body != nil && req.Body != http.NoBody) || req.Context().Err() != nil {
			return nil, err
		}
		next := nextProxyCandidate(candidates, candidate, time.Now())
		if next == nil {
			return nil, err
		}
		log.Infof("Failed to connect to request proxy %s, sending the request via %s instead: %v", candidate.name, next.name, err)
		candidate = next
	}
}

func (t *proxyFailoverTransport) acquireTransport(candidate *proxyCandidate) (*http.Transport, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.released {
		return nil, fmt.Errorf("transport has been released")
	}
	transport, releaser, err := t.cache.acquireTransport(candidate, t.localAddress, t.proxyProtocolAddrs)
	if err != nil {
		return nil, err
	}
	t.releasers = append(t.releasers, releaser)
	return transport, nil
}

func (t *proxyFailoverTransport) release() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.released = true
	for _, releaser := range t.releasers {
		releaser()
	}
	t.releasers = nil
}

func (c *HttpTransportCache) acquireTransport(candidate *proxyCandidate, localAddress net.IP, proxyProtocolAddrs *ProxyProtocolAddrs) (*http.Transport, TransportReleaser, error) {
	var proxyProtocolHeader *proxyproto.Header
	if candidate.proxyProtocolVersion != 0 {
		if proxyProtocolAddrs == nil {
			return nil, nil, fmt.Errorf("PROXY protocol addresses are required for proxy %s", candidate.name)
		}
		proxyProtocolHeader = NewProxyProtocolHeader(candidate.proxyProtocolVersion, proxyProtocolAddrs.Source, proxyProtocolAddrs.Destination)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.shutdown {
		return nil, nil, fmt.Errorf("HttpTransportCache has been shutdown")
	}

	key := strconv.Itoa(candidate.index) + "|"
	if localAddress == nil {
		key += "<nil>"
	} else {
		key += localAddress.String()
	}
	if proxyProtocolHeader != nil {
		key += "|" + proxyProtocolHeader.SourceAddr.String() + "|" + proxyProtocolHeader.DestinationAddr.String()
	}
	entry, ok := c.cache.Load(key)
	if !ok {
		entry = &transportEntry{
			localAddress: localAddress,
			transport:    newProxyCandidateTransport(candidate, localAddress, proxyProtocolHeader),
			lastUsed:     time.Now(),
			inUseCount:   0,
		}
		c.cache.Store(key, entry)
		log.Debugf("Created new transport for proxy %s, local address %q", candidate.name, localAddress)
	}

	entry.(*transportEntry).inUseCount++
	entry.(*transportEntry).lastUsed = time.Now()

	releaseFunc := func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		entry.(*transportEntry).inUseCount--
		entry.(*transportEntry).lastUsed = time.Now()
	}

	return entry.(*transportEntry).transport, releaseFunc, nil
}

func newProxyCandidateTransport(candidate *proxyCandidate, localAddress net.IP, proxyProtocolHeader *proxyproto.Header) *http.Transport {
	var localAddr net.Addr
	if localAddress != nil {
		localAddr = &net.TCPAddr{IP: localAddress}
	}
	dialer := &net.Dialer{
		Timeout:   candidate.dialTimeout,
		KeepAlive: 30 * time.Second,
		LocalAddr: localAddr,
	}

	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		if proxyProtocolHeader != nil {
			_ = conn.SetWriteDeadline(time.Now().Add(dialer.Timeout))
			_, err = proxyProtocolHeader.WriteTo(conn)
			_ = conn.SetWriteDeadline(time.Time{})
			if err != nil {
				_ = conn.Close()
				return nil, fmt.Errorf("failed to write PROXY protocol header to proxy: %w", err)
			}
		}
		return conn, nil
	}

	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dial(ctx, network, addr)
			if err != nil {
				if candidate.blockState != nil && ctx.Err() == nil {
					// a done ctx means the dial was abandoned by our side, so it's not the proxy's fault
					candidate.blockState.onDialFailure(time.Now(), err)
				}
				return nil, &proxyDialError{err: err}
			}
			if candidate.blockState != nil {
				candidate.blockState.onDialSuccess()
			}
			return conn, nil
		},
		Proxy: func(req *http.Request) (*url.URL, error) {
			return candidate.proxyUrl, nil
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   32,
		MaxConnsPerHost:       4,
	}
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
			log.Debugf("Removed idle transport for local address %q", key)
		}
	}

	for range c.houseKeeping.C {
		doHousekeeping()
	}
}
