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

// proxyBlockState tracks the TCP dial health of a request proxy, shared by all transports using that proxy.
//
// Only the TCP dial result (plus the optional PROXY protocol header write) is considered:
//   - dial success: reset everything
//   - dial failure (dial context not done): failures++, block the proxy for nextCooldown if failures reaches
//     the threshold and the proxy is not currently blocked, then double nextCooldown (capped by MaxCooldown)
//
// Since failures is only reset on success, the first dial after a block expires acts as a probe:
// one more failure blocks the proxy again immediately, with a doubled cooldown
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

// selectProxyCandidate returns the first non-blocked candidate in order.
// If all candidates are blocked, block states are ignored and the first one is returned
func selectProxyCandidate(candidates []*proxyCandidate, now time.Time) *proxyCandidate {
	if len(candidates) > 1 {
		for _, c := range candidates {
			if c.blockState == nil || !c.blockState.isBlocked(now) {
				return c
			}
		}
	}
	return candidates[0]
}
