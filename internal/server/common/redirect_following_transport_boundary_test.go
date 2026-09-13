package common

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	appctx "github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/stretchr/testify/require"
)

func TestRedirectFollowingTransportHonorsRedirectDecision(t *testing.T) {
	tests := []struct {
		name      string
		decision  RedirectDecision
		wantCalls int
		wantCode  int
		wantLoc   string
	}{
		{name: "follow", decision: RedirectDecisionFollow, wantCalls: 2, wantCode: http.StatusOK},
		{name: "return", decision: RedirectDecisionReturn, wantCalls: 1, wantCode: http.StatusFound, wantLoc: "http://upstream.test/final"},
		{name: "rewrite", decision: RedirectDecisionRewrite, wantCalls: 1, wantCode: http.StatusFound, wantLoc: "https://proxy.test/final"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"http://upstream.test/final"}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
				}

				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok")), Request: req}, nil
			})

			tr := NewRedirectFollowingTransport(appctx.NewRequestContext("test", "127.0.0.1"), transport, 3,
				func(*http.Response) *RedirectResult {
					return &RedirectResult{Decision: tc.decision, Value: tc.wantLoc}
				}, nil)

			resp, err := tr.RoundTrip((&http.Request{Method: http.MethodGet, URL: mustURL(t, "http://source.test/start"), Header: make(http.Header)}))

			require.NoError(t, err)
			require.Equal(t, tc.wantCode, resp.StatusCode)
			require.Equal(t, tc.wantCalls, calls)
			if tc.wantLoc != "" {
				require.Equal(t, tc.wantLoc, resp.Header.Get("Location"))
			}
		})
	}
}

func TestRedirectFollowingTransportStopsAtMaximum(t *testing.T) {
	calls := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"http://upstream.test/loop"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	})

	tr := NewRedirectFollowingTransport(appctx.NewRequestContext("test", "127.0.0.1"), transport, 2,
		func(*http.Response) *RedirectResult { return &RedirectResult{Decision: RedirectDecisionFollow} }, nil)
	resp, err := tr.RoundTrip((&http.Request{Method: http.MethodGet, URL: mustURL(t, "http://source.test/start"), Header: make(http.Header)}))

	require.NoError(t, err)
	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.Equal(t, 3, calls)
}

func TestRedirectRewritePreservesRelativeLocationComponents(t *testing.T) {
	dest := mustURL(t, "https://proxy.test/base")
	var cfg RunReverseProxyConfig
	WithRedirectRewriteOnly(dest, func(location *url.URL) bool {
		return location.Host == "upstream.test"
	})(&cfg)

	resp := &http.Response{
		StatusCode: http.StatusFound,
		Header:     http.Header{"Location": []string{"https://upstream.test/path?x=1#frag"}},
		Request:    &http.Request{URL: mustURL(t, "https://upstream.test/start")},
	}
	result := cfg.RedirectHandler(resp)

	require.Equal(t, RedirectDecision(RedirectDecisionRewrite), result.Decision)
	require.Equal(t, "https://proxy.test/path?x=1#frag", result.Value)
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}
