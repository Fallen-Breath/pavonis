package server

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testEventuallyTimeout = 5 * time.Second
const testEventuallyTick = 10 * time.Millisecond

func requireEventuallyServed(t *testing.T, front *httptest.Server, destination *testDestination, via string) {
	t.Helper()
	require.Eventually(t, func() bool {
		if sendTestRequest(front, "") != http.StatusOK {
			return false
		}
		requests := destination.Requests()
		return len(requests) > 0 && requests[len(requests)-1].via == via
	}, testEventuallyTimeout, testEventuallyTick, "requests are not served via %q", via)
}

func requireServed(t *testing.T, front *httptest.Server, destination *testDestination, via string) {
	t.Helper()
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusOK, sendTestRequest(front, ""))
		require.Equal(t, via, destination.Last(t).via)
	}
}

func TestRequestProxyPrefersFirstServer(t *testing.T) {
	destination := startTestDestination(t)
	p1 := startTestForwardProxy(t, "p1")
	p2 := startTestForwardProxy(t, "p2")
	front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: %s
  - url: %s
`, p1.URL, p2.URL), destination.URL)

	requireServed(t, front, destination, "p1")
	require.Empty(t, p2.Requests())
}

func TestRequestProxyFailsOverToNextServer(t *testing.T) {
	destination := startTestDestination(t)
	p2 := startTestForwardProxy(t, "p2")
	front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: http://%s
  - url: %s
`, unusedLocalAddr(t), p2.URL), destination.URL)

	requireEventuallyServed(t, front, destination, "p2")
	requireServed(t, front, destination, "p2")
}

func TestRequestProxyReturnsToRecoveredServer(t *testing.T) {
	destination := startTestDestination(t)
	p1Addr := unusedLocalAddr(t)
	p2 := startTestForwardProxy(t, "p2")
	front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: http://%s
  - url: %s
block:
  failure_threshold: 1
  cooldown: 100ms
  max_cooldown: 100ms
`, p1Addr, p2.URL), destination.URL)

	requireEventuallyServed(t, front, destination, "p2")

	listener, err := net.Listen("tcp", p1Addr)
	require.NoError(t, err)
	startTestForwardProxyOn(t, "p1", listener)

	requireEventuallyServed(t, front, destination, "p1")
	requireServed(t, front, destination, "p1")
}

func TestRequestProxyDirectFallback(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		destination := startTestDestination(t)
		front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: http://%s
  - url: http://%s
direct_fallback: true
`, unusedLocalAddr(t), unusedLocalAddr(t)), destination.URL)

		requireEventuallyServed(t, front, destination, "")
		requireServed(t, front, destination, "")
	})

	t.Run("disabled", func(t *testing.T) {
		destination := startTestDestination(t)
		front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: http://%s
  - url: http://%s
block:
  failure_threshold: 1
  cooldown: 1h
  max_cooldown: 1h
`, unusedLocalAddr(t), unusedLocalAddr(t)), destination.URL)

		for i := 0; i < 10; i++ {
			require.NotEqual(t, http.StatusOK, sendTestRequest(front, ""))
		}
		require.Empty(t, destination.Requests(), "destination is reached without any proxy")
	})
}

func TestRequestProxyOnlyConnectionFailuresTriggerFailover(t *testing.T) {
	httpDestination := startTestDestination(t).URL
	httpsDestination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(httpsDestination.Close)

	for _, tc := range []struct {
		name        string
		startProxy  func(t *testing.T) (string, func() int32)
		destination string
	}{
		{
			name: "error response",
			startProxy: func(t *testing.T) (string, func() int32) {
				proxyUrl, count := startTestStatusProxy(t, http.StatusBadGateway)
				return proxyUrl, count.Load
			},
			destination: httpDestination,
		},
		{
			name: "rejected CONNECT",
			startProxy: func(t *testing.T) (string, func() int32) {
				proxyUrl, count := startTestStatusProxy(t, http.StatusBadGateway)
				return proxyUrl, count.Load
			},
			destination: httpsDestination.URL,
		},
		{
			name: "connection closed",
			startProxy: func(t *testing.T) (string, func() int32) {
				proxyUrl, count := startTestClosingProxy(t)
				return proxyUrl, count.Load
			},
			destination: httpDestination,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			badProxy, badProxyConnections := tc.startProxy(t)
			p2 := startTestForwardProxy(t, "p2")
			front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: %s
  - url: %s
block:
  failure_threshold: 1
  cooldown: 1h
  max_cooldown: 1h
`, badProxy, p2.URL), tc.destination)

			for i := 0; i < 10; i++ {
				sendTestRequest(front, "")
			}
			require.NotZero(t, badProxyConnections(), "the first proxy is not used")
			require.Empty(t, p2.Requests(), "failed over to the next proxy")
		})
	}
}

func TestRequestProxyProxyProtocolFollowsSelectedServer(t *testing.T) {
	for _, tc := range []struct {
		name          string
		proxyProtocol string
		wantVersion   byte
	}{
		{name: "next server has its own version", proxyProtocol: "v1", wantVersion: 1},
		{name: "next server has it disabled", proxyProtocol: "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			destination := startTestDestination(t)
			p2 := startTestForwardProxy(t, "p2")
			front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: http://%s
    proxy_protocol: v2
  - url: %s
    proxy_protocol: %s
block:
  failure_threshold: 1
  cooldown: 1h
  max_cooldown: 1h
`, unusedLocalAddr(t), p2.URL, tc.proxyProtocol), destination.URL)

			requireEventuallyServed(t, front, destination, "p2")
			for _, req := range p2.Requests() {
				if tc.wantVersion == 0 {
					require.Nil(t, req.proxyHeader)
				} else {
					require.NotNil(t, req.proxyHeader)
					require.Equal(t, tc.wantVersion, req.proxyHeader.Version)
				}
			}
		})
	}

	t.Run("direct", func(t *testing.T) {
		destination := startTestDestination(t)
		front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: http://%s
    proxy_protocol: v2
direct_fallback: true
block:
  failure_threshold: 1
  cooldown: 1h
  max_cooldown: 1h
`, unusedLocalAddr(t)), destination.URL)

		requireEventuallyServed(t, front, destination, "")
		for _, req := range destination.Requests() {
			require.Nil(t, req.proxyHeader)
		}
	})
}
