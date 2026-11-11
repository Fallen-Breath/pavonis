package utils

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var errDialRejected = errors.New("dial rejected")

func rejectAllDials(netip.AddrPort) error {
	return errDialRejected
}

func newTestTransportCache(t *testing.T, proxyOpts RequestProxyOptions) *HttpTransportCache {
	cache := NewHttpTransportCache(16, time.Minute, proxyOpts)
	t.Cleanup(cache.Shutdown)
	return cache
}

func sendViaTransportCache(t *testing.T, cache *HttpTransportCache, opts TransportOptions, target string) error {
	transport, release := cache.GetTransport(opts)
	defer release()
	req, err := http.NewRequest(http.MethodGet, target, nil)
	require.NoError(t, err)
	resp, err := transport.RoundTrip(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	return err
}

func TestTransportCacheCheckDirectDialAddrSeesResolvedAddr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	target, err := url.Parse(server.URL)
	require.NoError(t, err)
	target.Host = "localhost:" + target.Port()

	var dialed []netip.AddrPort
	cache := newTestTransportCache(t, RequestProxyOptions{})
	err = sendViaTransportCache(t, cache, TransportOptions{CheckDirectDialAddr: func(addr netip.AddrPort) error {
		dialed = append(dialed, addr)
		return errDialRejected
	}}, target.String())

	require.ErrorIs(t, err, errDialRejected)
	require.NotEmpty(t, dialed)
	for _, addr := range dialed {
		require.True(t, addr.Addr().IsLoopback(), addr.String())
	}
}

func TestTransportCachePoolKeyIsolatesConnections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	cache := newTestTransportCache(t, RequestProxyOptions{})
	require.NoError(t, sendViaTransportCache(t, cache, TransportOptions{PoolKey: "a"}, server.URL))
	require.ErrorIs(t, sendViaTransportCache(t, cache, TransportOptions{PoolKey: "b", CheckDirectDialAddr: rejectAllDials}, server.URL), errDialRejected)
}

func TestTransportCacheCheckDirectDialAddrSkipsRequestProxies(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer proxy.Close()
	proxyUrl, err := url.Parse(proxy.URL)
	require.NoError(t, err)

	cache := newTestTransportCache(t, RequestProxyOptions{
		Servers:        []RequestProxyServerOptions{{Url: proxyUrl}},
		ConnectTimeout: time.Second,
		Block:          RequestProxyBlockOptions{FailureThreshold: 1, Cooldown: time.Second, MaxCooldown: time.Second},
	})
	require.NoError(t, sendViaTransportCache(t, cache, TransportOptions{CheckDirectDialAddr: rejectAllDials}, "http://example.com/"))
}
