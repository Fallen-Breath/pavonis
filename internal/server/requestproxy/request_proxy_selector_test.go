package requestproxy

import (
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestProxySelector(t *testing.T, strategy Strategy, weights ...int64) *proxySelector {
	t.Helper()
	opts := Options{
		Strategy:       strategy,
		ConnectTimeout: time.Second,
		Block:          BlockOptions{FailureThreshold: 1, Cooldown: time.Hour, MaxCooldown: time.Hour},
	}
	for i, weight := range weights {
		proxyUrl, err := url.Parse(fmt.Sprintf("http://proxy%d:8080", i))
		require.NoError(t, err)
		opts.Servers = append(opts.Servers, ServerOptions{Id: fmt.Sprintf("proxy%d", i), Url: proxyUrl, Weight: weight})
	}
	return newProxySelector(opts)
}

func generateTestClientKeys(n int) []string {
	var keys []string
	for i := 0; i < n; i++ {
		keys = append(keys, fmt.Sprintf("ipv4$10.%d.%d.1", i/256, i%256))
	}
	return keys
}

func TestProxySelectorRandomFollowsWeights(t *testing.T) {
	s := newTestProxySelector(t, StrategyRandom, 3000, 1000)
	first := 0
	for i := 0; i < 10000; i++ {
		if s.begin("").next(time.Now()).index == 0 {
			first++
		}
	}
	require.InDelta(t, 0.75, float64(first)/10000, 0.03)
}

func TestProxySelectorRandomFailoverFollowsRemainingWeights(t *testing.T) {
	s := newTestProxySelector(t, StrategyRandom, 4000, 3000, 1000)
	firstIsA, secondIsB := 0, 0
	for i := 0; i < 10000; i++ {
		selection := s.begin("")
		var tried []int
		for c := selection.next(time.Now()); c != nil; c = selection.next(time.Now()) {
			tried = append(tried, c.index)
		}
		require.ElementsMatch(t, []int{0, 1, 2}, tried)
		if tried[0] == 0 {
			firstIsA++
			if tried[1] == 1 {
				secondIsB++
			}
		}
	}
	require.InDelta(t, 0.75, float64(secondIsB)/float64(firstIsA), 0.05)
}

func TestProxySelectorIpHashFollowsWeights(t *testing.T) {
	s := newTestProxySelector(t, StrategyIpHash, 3000, 1000)
	keys := generateTestClientKeys(10000)
	first := 0
	for _, key := range keys {
		if s.begin(key).next(time.Now()).index == 0 {
			first++
		}
	}
	require.InDelta(t, 0.75, float64(first)/float64(len(keys)), 0.03)
}

func TestProxySelectorIpHashBlockedProxyKeepsClientOrder(t *testing.T) {
	s := newTestProxySelector(t, StrategyIpHash, 1000, 1000, 1000)
	keys := generateTestClientKeys(1000)
	// for clients preferring proxy 1, the proxy they fail over to within a request
	expected := make(map[string]int)
	for _, key := range keys {
		selection := s.begin(key)
		expected[key] = selection.next(time.Now()).index
		if expected[key] == 1 {
			expected[key] = selection.next(time.Now()).index
		}
	}

	now := time.Now()
	s.proxies[1].blockState.recordDialFailure(now, errors.New("dial failed"))
	for _, key := range keys {
		require.Equal(t, expected[key], s.begin(key).next(now).index)
	}
}
