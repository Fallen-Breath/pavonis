package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	proxyproto "github.com/pires/go-proxyproto"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// testViaHeader is added by testForwardProxy to forwarded requests, so the destination knows which proxy the request came through
const testViaHeader = "X-Test-Via"

type testConnKey struct{}

// withProxyProtocolListener makes the server accept connections with or without a PROXY protocol header,
// and exposes the connection in the request context, see receivedProxyHeader
func withProxyProtocolListener(server *httptest.Server) {
	server.Listener = &proxyproto.Listener{Listener: server.Listener}
	server.Config.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		return context.WithValue(ctx, testConnKey{}, conn)
	}
}

func receivedProxyHeader(r *http.Request) *proxyproto.Header {
	return r.Context().Value(testConnKey{}).(*proxyproto.Conn).ProxyHeader()
}

type testReceivedRequest struct {
	method      string
	via         string
	body        string
	proxyHeader *proxyproto.Header // nil if no PROXY protocol header is received
}

type testRequestRecorder struct {
	mu       sync.Mutex
	requests []testReceivedRequest
}

func (r *testRequestRecorder) record(req testReceivedRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)
}

func (r *testRequestRecorder) Requests() []testReceivedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]testReceivedRequest(nil), r.requests...)
}

func (r *testRequestRecorder) Last(t *testing.T) testReceivedRequest {
	t.Helper()
	requests := r.Requests()
	require.NotEmpty(t, requests, "no request received")
	return requests[len(requests)-1]
}

// testDestination is the site that pavonis proxies to
type testDestination struct {
	testRequestRecorder
	URL string
}

func startTestDestination(t *testing.T) *testDestination {
	t.Helper()
	d := &testDestination{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		d.record(testReceivedRequest{method: r.Method, via: r.Header.Get(testViaHeader), body: string(body), proxyHeader: receivedProxyHeader(r)})
		_, _ = io.WriteString(w, "ok")
	}))
	withProxyProtocolListener(server)
	server.Start()
	t.Cleanup(server.Close)
	d.URL = server.URL
	return d
}

// testForwardProxy is a working HTTP proxy, which accepts an optional PROXY protocol header
type testForwardProxy struct {
	testRequestRecorder
	URL string
}

func startTestForwardProxy(t *testing.T, name string) *testForwardProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return startTestForwardProxyOn(t, name, listener)
}

func startTestForwardProxyOn(t *testing.T, name string, listener net.Listener) *testForwardProxy {
	t.Helper()
	p := &testForwardProxy{}
	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.record(testReceivedRequest{method: r.Method, proxyHeader: receivedProxyHeader(r)})
		if r.Method == http.MethodConnect {
			serveTestConnectTunnel(w, r)
			return
		}
		r.RequestURI = ""
		r.Header.Set(testViaHeader, name)
		resp, err := transport.RoundTrip(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	_ = server.Listener.Close()
	server.Listener = listener
	withProxyProtocolListener(server)
	server.Start()
	t.Cleanup(server.Close)
	p.URL = server.URL
	return p
}

func serveTestConnectTunnel(w http.ResponseWriter, r *http.Request) {
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
}

// unusedLocalAddr returns a local address that nothing listens on, so connections to it are refused
func unusedLocalAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	return addr
}

// startTestStatusProxy starts a proxy that accepts connections, but responds every request (including CONNECT) with the given status
func startTestStatusProxy(t *testing.T, status int) (string, *atomic.Int32) {
	t.Helper()
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server.URL, &count
}

// startTestClosingProxy starts a proxy that accepts connections, then closes them immediately
func startTestClosingProxy(t *testing.T) (string, *atomic.Int32) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	var count atomic.Int32
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			count.Add(1)
			_ = conn.Close()
		}
	}()
	return "http://" + listener.Addr().String(), &count
}

// startTestPavonis starts pavonis with a single http site proxying to the destination.
// proxyYaml is the value of the request.proxy config
func startTestPavonis(t *testing.T, proxyYaml string, destination string) *httptest.Server {
	t.Helper()
	proxyYaml = strings.TrimSpace(proxyYaml)
	proxyYaml = "    " + strings.ReplaceAll(proxyYaml, "\n", "\n    ")
	var cfg config.Config
	require.NoError(t, yaml.Unmarshal([]byte(fmt.Sprintf(`
server:
  trusted_proxy_ips: [127.0.0.1/32]
  trusted_proxy_headers: [X-Forwarded-For]
request:
  proxy:
%s
sites:
  - mode: http
    host: '*'
    settings:
      destination: %s
      redirect_action: none
`, proxyYaml, destination)), &cfg))
	require.NoError(t, cfg.Init())
	s, err := NewPavonisServer(&cfg)
	require.NoError(t, err)
	t.Cleanup(s.Shutdown)
	front := httptest.NewServer(s)
	t.Cleanup(front.Close)
	return front
}

// sendTestRequest sends a GET request to pavonis, and returns the response status, or 0 if the request fails
func sendTestRequest(front *httptest.Server, forwardedFor string) int {
	req, err := http.NewRequest(http.MethodGet, front.URL+"/hello", nil)
	if err != nil {
		return 0
	}
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	return doTestRequest(front, req)
}

// sendTestBodyRequest sends a POST request with the given body to pavonis, and returns the response status, or 0 if the request fails
func sendTestBodyRequest(front *httptest.Server, body string) int {
	req, err := http.NewRequest(http.MethodPost, front.URL+"/hello", strings.NewReader(body))
	if err != nil {
		return 0
	}
	return doTestRequest(front, req)
}

func doTestRequest(front *httptest.Server, req *http.Request) int {
	client := front.Client()
	client.Timeout = 5 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode
}
