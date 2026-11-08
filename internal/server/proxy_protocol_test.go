package server

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func startTestPavonisWithProxyProtocol(t *testing.T, proxyUrl, proxyProtocolLine, destination string) *httptest.Server {
	t.Helper()
	return startTestPavonis(t, fmt.Sprintf("servers:\n  - url: %s\n    %s\n", proxyUrl, proxyProtocolLine), destination)
}

func requireProxyHeaderSourceIP(t *testing.T, req testReceivedRequest, wantIP string) {
	t.Helper()
	require.NotNil(t, req.proxyHeader)
	host, _, err := net.SplitHostPort(req.proxyHeader.SourceAddr.String())
	require.NoError(t, err)
	require.Equal(t, wantIP, host)
}

func TestProxyProxyProtocolRequests(t *testing.T) {
	for _, tc := range []struct {
		name              string
		proxyProtocolLine string
		forwardedIP       string
		wantVersion       byte
	}{
		{name: "omitted"},
		{name: "none", proxyProtocolLine: "proxy_protocol: none"},
		{name: "v1", proxyProtocolLine: "proxy_protocol: v1", wantVersion: 1},
		{name: "v2", proxyProtocolLine: "proxy_protocol: v2", wantVersion: 2},
		{name: "v2 forwarded", proxyProtocolLine: "proxy_protocol: v2", forwardedIP: "192.0.2.10", wantVersion: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			destination := startTestDestination(t)
			proxy := startTestForwardProxy(t, "p")
			front := startTestPavonisWithProxyProtocol(t, proxy.URL, tc.proxyProtocolLine, destination.URL)

			client := front.Client()
			client.Timeout = 5 * time.Second
			var clientAddr net.Addr
			req, err := http.NewRequest(http.MethodGet, front.URL+"/hello", nil)
			require.NoError(t, err)
			req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
				GotConn: func(info httptrace.GotConnInfo) { clientAddr = info.Conn.LocalAddr() },
			}))
			if tc.forwardedIP != "" {
				req.Header.Set("X-Forwarded-For", tc.forwardedIP)
			}
			resp, err := client.Do(req)
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, "ok", string(body))

			got := proxy.Last(t)
			if tc.wantVersion == 0 {
				require.Nil(t, got.proxyHeader)
				return
			}
			require.NotNil(t, got.proxyHeader)
			require.Equal(t, tc.wantVersion, got.proxyHeader.Version)
			if tc.forwardedIP != "" {
				requireProxyHeaderSourceIP(t, got, tc.forwardedIP)
			} else {
				require.Equal(t, clientAddr.String(), got.proxyHeader.SourceAddr.String())
			}
		})
	}
}

func TestProxyProxyProtocolKeepsClientIdentity(t *testing.T) {
	destination := startTestDestination(t)
	proxy := startTestForwardProxy(t, "p")
	front := startTestPavonisWithProxyProtocol(t, proxy.URL, "proxy_protocol: v2", destination.URL)

	for _, clientIP := range []string{"192.0.2.10", "192.0.2.20", "192.0.2.10"} {
		require.Equal(t, http.StatusOK, sendTestRequest(front, clientIP))
		requireProxyHeaderSourceIP(t, proxy.Last(t), clientIP)
	}
}

func TestProxyProxyProtocolHTTPSConnect(t *testing.T) {
	destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(destination.Close)
	proxy := startTestForwardProxy(t, "p")
	front := startTestPavonisWithProxyProtocol(t, proxy.URL, "proxy_protocol: v2", destination.URL)

	// the request itself fails, since pavonis does not trust the test certificate of the destination,
	// but the CONNECT request to the proxy has been sent before that
	sendTestRequest(front, "192.0.2.10")

	got := proxy.Last(t)
	require.Equal(t, http.MethodConnect, got.method)
	require.NotNil(t, got.proxyHeader)
	require.Equal(t, byte(2), got.proxyHeader.Version)
	requireProxyHeaderSourceIP(t, got, "192.0.2.10")
}
