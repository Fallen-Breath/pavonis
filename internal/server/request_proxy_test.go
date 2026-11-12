package server

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

// requireServed requires that requests are served via the given proxy ("" for direct) from the first one on,
// and that each request reaches the destination exactly once
func requireServed(t *testing.T, front *httptest.Server, destination *testDestination, via string) {
	t.Helper()
	for i := 0; i < 5; i++ {
		received := len(destination.Requests())
		require.Equal(t, http.StatusOK, sendTestRequest(front, ""))
		requests := destination.Requests()
		require.Len(t, requests, received+1, "the request is not received exactly once")
		require.Equal(t, via, requests[len(requests)-1].via)
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
	for _, scheme := range []string{"http", "https", "socks5", "socks5h"} {
		t.Run(scheme, func(t *testing.T) {
			destination := startTestDestination(t)
			p2 := startTestForwardProxy(t, "p2")
			front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: %s://%s
  - url: %s
`, scheme, unusedLocalAddr(t), p2.URL), destination.URL)

			requireServed(t, front, destination, "p2")
		})
	}
}

func TestRequestProxyFailsOverInPriorityOrder(t *testing.T) {
	destination := startTestDestination(t)
	p3 := startTestForwardProxy(t, "p3")
	p4 := startTestForwardProxy(t, "p4")
	front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: http://%s
  - url: http://%s
  - url: %s
  - url: %s
`, unusedLocalAddr(t), unusedLocalAddr(t), p3.URL, p4.URL), destination.URL)

	requireServed(t, front, destination, "p3")
	require.Empty(t, p4.Requests())
}

func TestRequestProxyFailsOverForHTTPSDestination(t *testing.T) {
	destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(destination.Close)
	p2 := startTestForwardProxy(t, "p2")
	front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: http://%s
  - url: %s
`, unusedLocalAddr(t), p2.URL), destination.URL)

	// the request itself fails, since pavonis does not trust the test certificate of the destination,
	// but the CONNECT request to the proxy has been sent before that
	sendTestRequest(front, "")

	requests := p2.Requests()
	require.Len(t, requests, 1)
	require.Equal(t, http.MethodConnect, requests[0].method)
}

func TestRequestProxyKeepsRequestBodyIntact(t *testing.T) {
	destination := startTestDestination(t)
	p2 := startTestForwardProxy(t, "p2")
	front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: http://%s
  - url: %s
`, unusedLocalAddr(t), p2.URL), destination.URL)

	// whether a failed request is sent again is not specified here,
	// but a request must never reach the destination more than once, or with a broken body
	served := 0
	for i := 0; i < 10; i++ {
		body := fmt.Sprintf("request body %d", i)
		if sendTestBodyRequest(front, body) == http.StatusOK {
			served++
			require.Equal(t, body, destination.Last(t).body)
		}
		require.Len(t, destination.Requests(), served)
	}
}

func TestRequestProxySkipsBlockedServer(t *testing.T) {
	destination := startTestDestination(t)
	p2 := startTestForwardProxy(t, "p2")
	front := startTestPavonis(t, fmt.Sprintf(`
servers:
  - url: http://%s
  - url: %s
block:
  failure_threshold: 1
  cooldown: 1h
  max_cooldown: 1h
`, unusedLocalAddr(t), p2.URL), destination.URL)

	requireServed(t, front, destination, "p2")

	// the first server is blocked now, so it is not even tried
	for i := 0; i < 5; i++ {
		body := fmt.Sprintf("request body %d", i)
		require.Equal(t, http.StatusOK, sendTestBodyRequest(front, body))
		got := destination.Last(t)
		require.Equal(t, "p2", got.via)
		require.Equal(t, body, got.body)
	}
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

	requireServed(t, front, destination, "p2")

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

	startBadGatewayProxy := func(t *testing.T) (string, *atomic.Int32) { return startTestStatusProxy(t, http.StatusBadGateway) }
	for _, tc := range []struct {
		name        string
		startProxy  func(t *testing.T) (string, *atomic.Int32)
		destination string
	}{
		{"error response", startBadGatewayProxy, httpDestination},
		{"rejected CONNECT", startBadGatewayProxy, httpsDestination.URL},
		{"connection closed", startTestClosingProxy, httpDestination},
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
			require.NotZero(t, badProxyConnections.Load(), "the first proxy is not used")
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
`, unusedLocalAddr(t), p2.URL, tc.proxyProtocol), destination.URL)

			requireServed(t, front, destination, "p2")
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
`, unusedLocalAddr(t)), destination.URL)

		requireServed(t, front, destination, "")
		for _, req := range destination.Requests() {
			require.Nil(t, req.proxyHeader)
		}
	})
}
