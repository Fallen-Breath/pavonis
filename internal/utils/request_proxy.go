package utils

import (
	"net/url"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type RequestProxyServerOptions struct {
	Url                  *url.URL
	ProxyProtocolVersion byte // 0 means PROXY protocol is disabled
}

type RequestProxyBlockOptions struct {
	FailureThreshold int
	Cooldown         time.Duration
	MaxCooldown      time.Duration
}

type RequestProxyOptions struct {
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

// proxyCandidate is a choice of how requests reach the destination: via a proxy, or directly (proxyUrl == nil)
type proxyCandidate struct {
	index                int
	name                 string
	proxyUrl             *url.URL
	proxyProtocolVersion byte
	dialTimeout          time.Duration
	blockState           *proxyBlockState // nil for the direct candidate, which is never blocked
}

func newProxyCandidates(opts RequestProxyOptions) []*proxyCandidate {
	var candidates []*proxyCandidate
	for _, proxy := range opts.Servers {
		name := proxy.Url.Redacted()
		candidates = append(candidates, &proxyCandidate{
			index:                len(candidates),
			name:                 name,
			proxyUrl:             proxy.Url,
			proxyProtocolVersion: proxy.ProxyProtocolVersion,
			dialTimeout:          opts.ConnectTimeout,
			blockState:           newProxyBlockState(name, opts.Block),
		})
	}
	if len(candidates) == 0 || opts.DirectFallback {
		candidates = append(candidates, &proxyCandidate{
			index:       len(candidates),
			name:        "<direct>",
			dialTimeout: 10 * time.Second,
		})
	}
	return candidates
}

func (c *proxyCandidate) isBlocked(now time.Time) bool {
	return c.blockState != nil && c.blockState.isBlocked(now)
}

func selectProxyCandidate(candidates []*proxyCandidate, now time.Time) *proxyCandidate {
	for _, c := range candidates {
		if !c.isBlocked(now) {
			return c
		}
	}
	return candidates[0]
}

func nextProxyCandidate(candidates []*proxyCandidate, current *proxyCandidate, now time.Time) *proxyCandidate {
	for _, c := range candidates[current.index+1:] {
		if !c.isBlocked(now) {
			return c
		}
	}
	return nil
}

// proxyDialError marks errors returned by our DialContext, so that RoundTrip errors caused by them can be recognized.
// http.Transport keeps the dial error in the error chain, but this is not documented.
// If that ever changes, requests are just not resent via another candidate
type proxyDialError struct {
	err error
}

func (e *proxyDialError) Error() string {
	return e.err.Error()
}

func (e *proxyDialError) Unwrap() error {
	return e.err
}
