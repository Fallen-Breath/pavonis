package utils

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"time"
)

type proxySelector struct {
	strategy RequestProxyStrategy
	proxies  []*proxyCandidate
	direct   *proxyCandidate // nil if direct connections are not allowed
}

func newProxySelector(opts RequestProxyOptions) *proxySelector {
	s := &proxySelector{strategy: opts.Strategy}
	for _, proxy := range opts.Servers {
		name := proxy.Url.Redacted()
		s.proxies = append(s.proxies, &proxyCandidate{
			index:                len(s.proxies),
			name:                 name,
			id:                   proxy.Id,
			weight:               proxy.Weight,
			proxyUrl:             proxy.Url,
			proxyProtocolVersion: proxy.ProxyProtocolVersion,
			dialTimeout:          opts.ConnectTimeout,
			blockState:           newProxyBlockState(name, opts.Block),
		})
	}
	if len(s.proxies) == 0 || opts.DirectFallback {
		s.direct = &proxyCandidate{
			index:       len(s.proxies),
			name:        "<direct>",
			dialTimeout: 10 * time.Second,
		}
	}
	return s
}

// begin starts choosing candidates for one outgoing request.
// With the ip_hash strategy, the same clientKey always gets the same proxy order
func (s *proxySelector) begin(clientKey string) *proxySelection {
	return &proxySelection{
		selector:  s,
		clientKey: clientKey,
		tried:     make([]bool, len(s.proxies)),
	}
}

type proxySelection struct {
	selector   *proxySelector
	clientKey  string
	started    bool
	exhausted  bool
	cursor     int    // by_order only, the index to continue from
	tried      []bool // random and ip_hash only
	hashScores []float64
}

// next returns the candidate to try next, or nil if there's none left.
// Each candidate is returned at most once, blocked proxies are skipped, and the direct candidate always comes last
func (sel *proxySelection) next(now time.Time) *proxyCandidate {
	s := sel.selector
	first := !sel.started
	sel.started = true
	if sel.exhausted {
		return nil
	}
	if proxy := sel.selectNextProxy(now); proxy != nil {
		return proxy
	}

	sel.exhausted = true
	if s.direct != nil {
		return s.direct
	}
	if first {
		// all proxies are blocked, and there's no direct fallback. Still make one attempt instead of failing right away
		return sel.pick(s.proxies)
	}
	return nil
}

func (sel *proxySelection) selectNextProxy(now time.Time) *proxyCandidate {
	s := sel.selector
	if s.strategy == RequestProxyStrategyByOrder {
		// never go back, even if a skipped proxy has been unblocked meanwhile
		for _, proxy := range s.proxies[sel.cursor:] {
			if !proxy.isBlocked(now) {
				sel.cursor = proxy.index + 1
				return proxy
			}
		}
		sel.cursor = len(s.proxies)
		return nil
	}

	var available []*proxyCandidate
	for _, proxy := range s.proxies {
		if !sel.tried[proxy.index] && !proxy.isBlocked(now) {
			available = append(available, proxy)
		}
	}
	if len(available) == 0 {
		return nil
	}
	proxy := sel.pick(available)
	sel.tried[proxy.index] = true
	return proxy
}

func (sel *proxySelection) pick(proxies []*proxyCandidate) *proxyCandidate {
	switch sel.selector.strategy {
	case RequestProxyStrategyRandom:
		var total int64
		for _, proxy := range proxies {
			total += proxy.weight
		}
		r := rand.Int64N(total)
		for _, proxy := range proxies {
			if r < proxy.weight {
				return proxy
			}
			r -= proxy.weight
		}
		panic("unreachable")
	case RequestProxyStrategyIpHash:
		if sel.hashScores == nil {
			sel.hashScores = make([]float64, len(sel.selector.proxies))
			for _, proxy := range sel.selector.proxies {
				sel.hashScores[proxy.index] = calculateWeightedRendezvousScore(sel.clientKey, proxy)
			}
		}
		best := proxies[0]
		for _, proxy := range proxies[1:] {
			if sel.hashScores[proxy.index] > sel.hashScores[best.index] {
				best = proxy
			}
		}
		return best
	default:
		return proxies[0]
	}
}

// calculateWeightedRendezvousScore uses the logarithmic method of weighted rendezvous hashing.
// A proxy's score depends only on the key and the proxy itself, so removing a proxy keeps the order of the others.
// See https://datatracker.ietf.org/doc/html/draft-ietf-bess-weighted-hrw-02
func calculateWeightedRendezvousScore(key string, proxy *proxyCandidate) float64 {
	hash := sha256.Sum256([]byte("pavonis:" + key + "|" + proxy.id))
	// uniform in (0, 1), excluding both ends
	u := (float64(binary.BigEndian.Uint64(hash[:8])>>11) + 0.5) / (1 << 53)
	return -float64(proxy.weight) / math.Log(u)
}
