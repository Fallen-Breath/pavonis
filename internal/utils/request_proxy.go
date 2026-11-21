package utils

import (
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type RequestProxyStrategy int

const (
	RequestProxyStrategyByOrder RequestProxyStrategy = iota
	RequestProxyStrategyRandom
	RequestProxyStrategyIpHash
)

type RequestProxyServerOptions struct {
	Url                  *url.URL
	ProxyProtocolVersion byte // 0 means PROXY protocol is disabled
	Weight               int64
}

type RequestProxyBlockOptions struct {
	FailureThreshold int
	Cooldown         time.Duration
	MaxCooldown      time.Duration
}

type RequestProxyOptions struct {
	Strategy       RequestProxyStrategy
	Servers        []RequestProxyServerOptions
	DirectFallback bool
	ConnectTimeout time.Duration
	Block          RequestProxyBlockOptions
}

// failures is only reset by a successful dial, so after a block expires,
// a single failure blocks the proxy again, with a doubled cooldown
type proxyBlockState struct {
	name string
	opts RequestProxyBlockOptions

	mu           sync.Mutex
	failures     int
	blockedUntil time.Time
	nextCooldown time.Duration
}

func newProxyBlockState(name string, opts RequestProxyBlockOptions) *proxyBlockState {
	return &proxyBlockState{
		name:         name,
		opts:         opts,
		nextCooldown: opts.Cooldown,
	}
}

func (s *proxyBlockState) isBlocked(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return now.Before(s.blockedUntil)
}

func (s *proxyBlockState) onDialSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failures >= s.opts.FailureThreshold {
		log.Infof("Request proxy %s is reachable again, unblocked", s.name)
	}
	s.failures = 0
	s.blockedUntil = time.Time{}
	s.nextCooldown = s.opts.Cooldown
}

func (s *proxyBlockState) onDialFailure(now time.Time, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures++
	log.Debugf("Request proxy %s dial failed (consecutive failures: %d): %v", s.name, s.failures, err)
	if s.failures >= s.opts.FailureThreshold && !now.Before(s.blockedUntil) {
		cooldown := s.nextCooldown
		s.blockedUntil = now.Add(cooldown)
		s.nextCooldown = min(2*cooldown, s.opts.MaxCooldown)
		log.Warnf("Request proxy %s blocked for %s after %d consecutive dial failures, last error: %v", s.name, cooldown, s.failures, err)
	}
}

// RequestProxyIdentity tells whether two proxy urls are the same proxy.
// The password is excluded, so that rotating it keeps the ip_hash mapping
func RequestProxyIdentity(proxyUrl *url.URL) string {
	scheme := strings.ToLower(proxyUrl.Scheme)
	port := proxyUrl.Port()
	if portNumber, err := strconv.Atoi(port); err == nil {
		port = strconv.Itoa(portNumber)
	} else {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		case "socks5", "socks5h":
			port = "1080"
		}
	}
	host := strings.ToLower(proxyUrl.Hostname())
	if addr, err := netip.ParseAddr(host); err == nil {
		host = addr.String()
	}

	identity := scheme + "://"
	if proxyUrl.User != nil {
		identity += url.QueryEscape(proxyUrl.User.Username()) + "@"
	}
	return identity + net.JoinHostPort(host, port)
}

// proxyCandidate is a choice of how requests reach the destination: via a proxy, or directly (proxyUrl == nil)
type proxyCandidate struct {
	index                int
	name                 string
	identity             string
	weight               int64
	proxyUrl             *url.URL
	proxyProtocolVersion byte
	dialTimeout          time.Duration
	blockState           *proxyBlockState // nil for the direct candidate, which is never blocked
}

func (c *proxyCandidate) isBlocked(now time.Time) bool {
	return c.blockState != nil && c.blockState.isBlocked(now)
}

// proxyDialError marks errors returned by our DialContext, so that RoundTrip errors caused by them can be recognized.
// http.Transport keeps the dial error in the error chain, but this is not documented.
// If that ever changes, requests are just not resent via another candidate
type proxyDialError struct {
	err error
}

var _ error = &proxyDialError{}

func (e *proxyDialError) Error() string {
	return e.err.Error()
}

func (e *proxyDialError) Unwrap() error {
	return e.err
}
