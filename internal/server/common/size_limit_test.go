package common

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	appctx "github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/stretchr/testify/require"
)

// onlyReader hides other interfaces, so the http client cannot know the body size and sends it chunked
type onlyReader struct{ io.Reader }

type sizeLimitUpstream struct {
	mu       sync.Mutex
	received []string // request bodies fully received by the upstream
}

func (u *sizeLimitUpstream) Received() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]string(nil), u.received...)
}

// startSizeLimitTest starts an upstream that responds with a body of /{size} bytes, chunked if the query has "chunked",
// and a proxy server in front of it with the given options
func startSizeLimitTest(t *testing.T, opts ...ReverseProxyOption) (*sizeLimitUpstream, string) {
	upstream := &sizeLimitUpstream{}
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}
		upstream.mu.Lock()
		upstream.received = append(upstream.received, string(body))
		upstream.mu.Unlock()

		size, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
		if !r.URL.Query().Has("chunked") {
			w.Header().Set("Content-Length", strconv.Itoa(size))
		}
		for i := 0; i < size; i += 10 {
			_, _ = w.Write(bytes.Repeat([]byte("a"), min(10, size-i)))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	t.Cleanup(upstreamServer.Close)
	dest, err := url.Parse(upstreamServer.URL)
	require.NoError(t, err)

	helper := newTestRequestHelper(t)
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := *dest
		target.Path = r.URL.Path
		target.RawQuery = r.URL.RawQuery
		helper.RunReverseProxy(appctx.NewRequestContext("test", "127.0.0.1"), w, r, &target, opts...)
	}))
	t.Cleanup(proxyServer.Close)
	return upstream, proxyServer.URL
}

func TestResponseSizeLimit(t *testing.T) {
	_, proxyUrl := startSizeLimitTest(t, WithResponseSizeLimit(100))

	for _, tc := range []struct {
		name string
		path string
	}{
		{"known length within limit", "/100"},
		{"chunked within limit", "/100?chunked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(proxyUrl + tc.path)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Len(t, body, 100)
		})
	}

	t.Run("known length over limit", func(t *testing.T) {
		resp, err := http.Get(proxyUrl + "/101")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("chunked over limit", func(t *testing.T) {
		resp, err := http.Get(proxyUrl + "/1000?chunked")
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		// the client must not see a complete response
		require.Error(t, err)
		require.LessOrEqual(t, len(body), 100)
	})
}

func TestRequestSizeLimit(t *testing.T) {
	upstream, proxyUrl := startSizeLimitTest(t, WithRequestSizeLimit(100))

	post := func(body io.Reader) *http.Response {
		resp, err := http.Post(proxyUrl+"/0", "application/octet-stream", body)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return resp
	}

	within := strings.Repeat("b", 100)
	require.Equal(t, http.StatusOK, post(strings.NewReader(within)).StatusCode)
	require.Equal(t, http.StatusOK, post(onlyReader{strings.NewReader(within)}).StatusCode)
	require.Equal(t, []string{within, within}, upstream.Received())

	over := strings.Repeat("b", 1000)
	require.Equal(t, http.StatusRequestEntityTooLarge, post(strings.NewReader(over)).StatusCode)
	require.Equal(t, http.StatusRequestEntityTooLarge, post(onlyReader{strings.NewReader(over)}).StatusCode)
	// the upstream never receives a complete request that is over the limit
	require.Equal(t, []string{within, within}, upstream.Received())
}
