package common

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	appctx "github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/stretchr/testify/require"
)

func TestRunReverseProxyAppliesHeaderModifications(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "rewritten", r.Header.Get("X-Test"))
		require.Empty(t, r.Header.Get("X-Remove"))
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	helper := newTestRequestHelper(t)
	modifyReq := map[string]string{"X-Test": "rewritten"}
	deleteReq := []string{"X-Remove"}
	modifyResp := map[string]string{"X-Response": "rewritten"}
	deleteResp := []string{"X-Upstream"}
	helper.cfg.Request.Header.Modify = &modifyReq
	helper.cfg.Request.Header.Delete = &deleteReq
	helper.cfg.Response.Header.Modify = &modifyResp
	helper.cfg.Response.Header.Delete = &deleteResp

	req := httptest.NewRequest(http.MethodGet, "/source", nil)
	req.Header.Set("X-Test", "original")
	req.Header.Set("X-Remove", "present")
	rec := httptest.NewRecorder()
	dest, err := url.Parse(upstream.URL)
	require.NoError(t, err)

	helper.RunReverseProxy(appctx.NewRequestContext("test", "127.0.0.1"), rec, req, dest)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "rewritten", rec.Header().Get("X-Response"))
	require.Empty(t, rec.Header().Get("X-Upstream"))
	require.Equal(t, "ok", rec.Body.String())
}

func TestCreateErrorHandlerMapsErrors(t *testing.T) {
	helper := newTestRequestHelper(t)
	h := helper.createErrorHandler(appctx.NewRequestContext("test", "127.0.0.1"))

	tests := []struct {
		name string
		err  error
		code int
		body string
	}{
		{"timeout", context.DeadlineExceeded, http.StatusGatewayTimeout, "request timed out\n"},
		{"http error", NewHttpError(http.StatusForbidden, "blocked"), http.StatusForbidden, "blocked\n"},
		{"unknown", errors.New("upstream failed"), http.StatusBadGateway, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h(rec, httptest.NewRequest(http.MethodGet, "/", nil), tc.err)
			require.Equal(t, tc.code, rec.Code)
			if tc.body != "" {
				require.Equal(t, tc.body, rec.Body.String())
			}
		})
	}
}

func TestRedirectFollowingTransportRebuildsRequestsAndStripsSensitiveHeaders(t *testing.T) {
	var seen []*http.Request
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seen = append(seen, req)
		if len(seen) == 1 {
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"http://other.test/final"}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	})
	tr := NewRedirectFollowingTransport(appctx.NewRequestContext("test", "127.0.0.1"), transport, 2, func(*http.Response) *RedirectResult { return &RedirectResult{Decision: RedirectDecisionFollow} }, nil)
	req := httptest.NewRequest(http.MethodGet, "http://source.test/start", nil)
	req.Header.Set("Authorization", "secret")
	req.Header.Set("X-Test", "keep")
	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, seen, 2)
	require.Empty(t, seen[1].Header.Get("Authorization"))
	require.Equal(t, "keep", seen[1].Header.Get("X-Test"))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newTestRequestHelper(t *testing.T) *RequestHelper {
	t.Helper()
	cfg := testConfig()
	factory, err := NewRequestHelperFactory(cfg)
	require.NoError(t, err)
	t.Cleanup(factory.Shutdown)
	return factory.NewRequestHelper(nil)
}

func testConfig() *config.Config {
	cfg := &config.Config{}
	if err := cfg.Init(); err != nil {
		panic(err)
	}
	return cfg
}
