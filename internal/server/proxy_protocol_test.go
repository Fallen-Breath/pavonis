package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/url"
	"testing"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	proxyproto "github.com/pires/go-proxyproto"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestProxyProxyProtocolRequests(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "upstream:"+r.URL.Path)
	}))
	t.Cleanup(upstream.Close)

	for _, tc := range []struct {
		version     string
		forwardedIP string
		wantVersion byte
	}{
		{version: "none"},
		{version: "v1", wantVersion: 1},
		{version: "v2", forwardedIP: "192.0.2.10", wantVersion: 2},
	} {
		t.Run(tc.version, func(t *testing.T) {
			proxyURL, requests := newProxyProtocolTestProxy(t)
			front := newProxyProtocolTestServer(t, proxyURL, tc.version, upstream.URL)
			client := front.Client()
			client.Timeout = 5 * time.Second
			clientAddr := make(chan net.Addr, 1)
			req, err := http.NewRequest(http.MethodGet, front.URL+"/hello", nil)
			require.NoError(t, err)
			req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
				GotConn: func(info httptrace.GotConnInfo) { clientAddr <- info.Conn.LocalAddr() },
			}))
			if tc.forwardedIP != "" {
				req.Header.Set("X-Forwarded-For", tc.forwardedIP)
			}
			resp, err := client.Do(req)
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			require.NoError(t, err)
			require.Equal(t, http.StatusAccepted, resp.StatusCode)
			require.Equal(t, "upstream:/hello", string(body))
			require.NotEmpty(t, requests, "proxy received no request")
			got := <-requests
			if tc.wantVersion == 0 {
				require.Nil(t, got.header)
				return
			}
			require.NotNil(t, got.header)
			require.Equal(t, tc.wantVersion, got.header.Version)
			wantSource := (<-clientAddr).String()
			if tc.forwardedIP != "" {
				wantSource = net.JoinHostPort(tc.forwardedIP, "0")
			}
			require.Equal(t, wantSource, got.header.SourceAddr.String())
		})
	}
}

func TestProxyProxyProtocolKeepsClientIdentity(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(upstream.Close)
	proxyURL, requests := newProxyProtocolTestProxy(t)
	front := newProxyProtocolTestServer(t, proxyURL, "v2", upstream.URL)
	client := front.Client()
	client.Timeout = 5 * time.Second

	for _, clientIP := range []string{"192.0.2.10", "192.0.2.20", "192.0.2.10"} {
		req, err := http.NewRequest(http.MethodGet, front.URL+"/hello", nil)
		require.NoError(t, err)
		req.Header.Set("X-Forwarded-For", clientIP)
		resp, err := client.Do(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "ok", string(body))
		require.NotEmpty(t, requests, "proxy received no request")
		got := <-requests
		require.NotNil(t, got.header)
		require.Equal(t, net.JoinHostPort(clientIP, "0"), got.header.SourceAddr.String())
	}
}

func TestProxyProxyProtocolHTTPSConnect(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "https upstream")
	}))
	t.Cleanup(upstream.Close)
	proxyURL, requests := newProxyProtocolTestProxy(t)
	proxyAddr, err := url.Parse(proxyURL)
	require.NoError(t, err)
	cache := utils.NewHttpTransportCache(4, time.Minute, proxyAddr, "v2")
	t.Cleanup(cache.Shutdown)
	header := proxyproto.HeaderProxyFromAddrs(2,
		&net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 12345},
		&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8009})
	transport, release := cache.GetTransport(nil, header)
	t.Cleanup(release)
	t.Cleanup(transport.CloseIdleConnections)
	transport.TLSClientConfig = upstream.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	resp, err := client.Get(upstream.URL)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "https upstream", string(body))
	require.NotEmpty(t, requests, "proxy received no request")
	got := <-requests
	require.Equal(t, http.MethodConnect, got.method)
	require.NotNil(t, got.header)
	require.Equal(t, byte(2), got.header.Version)
	require.Equal(t, "192.0.2.10:12345", got.header.SourceAddr.String())
}

func newProxyProtocolTestServer(t *testing.T, proxyURL, version, upstreamURL string) *httptest.Server {
	t.Helper()
	var cfg config.Config
	require.NoError(t, yaml.Unmarshal([]byte(fmt.Sprintf(`
server:
  trusted_proxy_ips: [127.0.0.1/32]
  trusted_proxy_headers: [X-Forwarded-For]
request:
  proxy: %s
  proxy_proxy_protocol: %s
sites:
  - mode: http
    host: '*'
    settings:
      destination: %s
      redirect_action: none
`, proxyURL, version, upstreamURL)), &cfg))
	require.NoError(t, cfg.Init())
	s, err := NewPavonisServer(&cfg)
	require.NoError(t, err)
	t.Cleanup(s.Shutdown)
	front := httptest.NewServer(s)
	t.Cleanup(front.Close)
	return front
}

type proxyProtocolTestConnKey struct{}

type proxyProtocolTestRequest struct {
	header *proxyproto.Header
	method string
}

func newProxyProtocolTestProxy(t *testing.T) (string, <-chan proxyProtocolTestRequest) {
	t.Helper()
	requests := make(chan proxyProtocolTestRequest, 4)
	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn := r.Context().Value(proxyProtocolTestConnKey{}).(*proxyproto.Conn)
		requests <- proxyProtocolTestRequest{header: conn.ProxyHeader(), method: r.Method}
		if r.Method == http.MethodConnect {
			upstream, err := net.Dial("tcp", r.Host)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			defer upstream.Close()
			tunnel, buffered, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			defer tunnel.Close()
			_, _ = io.WriteString(tunnel, "HTTP/1.1 200 Connection Established\r\n\r\n")
			go func() {
				_, _ = io.Copy(upstream, buffered)
				_ = upstream.Close()
			}()
			_, _ = io.Copy(tunnel, upstream)
			return
		}
		r.RequestURI = ""
		resp, err := transport.RoundTrip(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	proxy.Listener = &proxyproto.Listener{Listener: proxy.Listener}
	proxy.Config.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		return context.WithValue(ctx, proxyProtocolTestConnKey{}, conn)
	}
	proxy.Start()
	t.Cleanup(proxy.Close)
	return proxy.URL, requests
}
