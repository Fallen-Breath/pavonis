package common

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	appctx "github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/stretchr/testify/require"
)

func redirectResponse(req *http.Request, status int, location string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Location": {location}}, Body: http.NoBody, Request: req}
}

func newRedirectFollowingTransport(transport http.RoundTripper, maxRedirect int, decision RedirectDecision, value string) http.RoundTripper {
	return NewRedirectFollowingTransport(appctx.NewRequestContext("test", "127.0.0.1"), transport, maxRedirect,
		func(*http.Response) *RedirectResult { return &RedirectResult{Decision: decision, Value: value} }, nil)
}

func TestRedirectFollowingTransportHonorsRedirectDecision(t *testing.T) {
	for _, tc := range []struct {
		name      string
		decision  RedirectDecision
		value     string
		wantCalls int
		wantCode  int
		wantLoc   string
	}{
		{name: "follow", decision: RedirectDecisionFollow, wantCalls: 2, wantCode: http.StatusOK},
		{name: "return", decision: RedirectDecisionReturn, wantCalls: 1, wantCode: http.StatusFound, wantLoc: "http://upstream.test/final"},
		{name: "rewrite", decision: RedirectDecisionRewrite, value: "https://proxy.test/final", wantCalls: 1, wantCode: http.StatusFound, wantLoc: "https://proxy.test/final"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return redirectResponse(req, http.StatusFound, "http://upstream.test/final"), nil
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
			})

			resp, err := newRedirectFollowingTransport(transport, 3, tc.decision, tc.value).RoundTrip(httptest.NewRequest(http.MethodGet, "http://source.test/start", nil))
			require.NoError(t, err)
			require.Equal(t, tc.wantCode, resp.StatusCode)
			require.Equal(t, tc.wantCalls, calls)
			if tc.wantLoc != "" {
				require.Equal(t, tc.wantLoc, resp.Header.Get("Location"))
			}
		})
	}
}

func TestRedirectFollowingTransportAbortsWithError(t *testing.T) {
	calls := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return redirectResponse(req, http.StatusFound, "http://upstream.test/final"), nil
	})
	abortErr := NewHttpError(http.StatusForbidden, "forbidden")
	tr := NewRedirectFollowingTransport(appctx.NewRequestContext("test", "127.0.0.1"), transport, 3,
		func(*http.Response) *RedirectResult {
			return &RedirectResult{Decision: RedirectDecisionAbort, Err: abortErr}
		}, nil)

	_, err := tr.RoundTrip(httptest.NewRequest(http.MethodGet, "http://source.test/start", nil))
	require.ErrorIs(t, err, abortErr)
	require.Equal(t, 1, calls)
}

func TestRedirectFollowingTransportStopsAtMaximum(t *testing.T) {
	calls := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return redirectResponse(req, http.StatusFound, "http://upstream.test/loop"), nil
	})

	resp, err := newRedirectFollowingTransport(transport, 2, RedirectDecisionFollow, "").RoundTrip(httptest.NewRequest(http.MethodGet, "http://source.test/start", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.Equal(t, 3, calls)
}

func TestRedirectFollowingTransportStripsSensitiveHeaders(t *testing.T) {
	var seen []*http.Request
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seen = append(seen, req)
		if len(seen) == 1 {
			return redirectResponse(req, http.StatusFound, "http://other.test/final"), nil
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
	})
	req := httptest.NewRequest(http.MethodGet, "http://source.test/start", nil)
	req.Header.Set("Authorization", "secret")
	req.Header.Set("X-Test", "keep")

	resp, err := newRedirectFollowingTransport(transport, 2, RedirectDecisionFollow, "").RoundTrip(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, seen, 2)
	require.Empty(t, seen[1].Header.Get("Authorization"))
	require.Equal(t, "keep", seen[1].Header.Get("X-Test"))
}

func TestRedirectFollowingTransportRedirectMethodAndBody(t *testing.T) {
	const payload = "request payload"
	type request struct{ method, body string }
	for _, tc := range []struct {
		name  string
		codes []int
		want  []request
	}{
		{"POST 302", []int{302}, []request{{"POST", payload}, {"GET", ""}}},
		{"PUT 302", []int{302}, []request{{"PUT", payload}, {"PUT", payload}}},
		{"PUT 303", []int{303}, []request{{"PUT", payload}, {"GET", ""}}},
		{"HEAD 303", []int{303}, []request{{"HEAD", ""}, {"HEAD", ""}}},
		{"POST 307", []int{307}, []request{{"POST", payload}, {"POST", payload}}},
		{"POST 307 then 303", []int{307, 303}, []request{{"POST", payload}, {"POST", payload}, {"GET", ""}}},
		{"POST 303 then 307", []int{303, 307}, []request{{"POST", payload}, {"GET", ""}, {"GET", ""}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var received []request
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var body []byte
				if req.Body != nil {
					defer req.Body.Close()
					var err error
					if body, err = io.ReadAll(req.Body); err != nil {
						return nil, err
					}
				}
				step := len(received)
				received = append(received, request{req.Method, string(body)})
				if step < len(tc.codes) {
					return redirectResponse(req, tc.codes[step], "/next"), nil
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
			})
			req := httptest.NewRequest(tc.want[0].method, "http://upstream.test/start", strings.NewReader(tc.want[0].body))

			resp, err := newRedirectFollowingTransport(transport, 3, RedirectDecisionFollow, "").RoundTrip(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, tc.want, received)
		})
	}
}

func TestRedirectFollowingTransportRejectsFailedBodyReplay(t *testing.T) {
	followed := false
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		defer req.Body.Close()
		if req.URL.Path == "/final" {
			followed = true
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
		}
		// The upstream redirects even though reading the upload has failed
		_, _ = io.Copy(io.Discard, req.Body)
		return redirectResponse(req, http.StatusTemporaryRedirect, "/final"), nil
	})
	body := io.MultiReader(strings.NewReader("partial payload"), iotest.ErrReader(io.ErrUnexpectedEOF))
	req := httptest.NewRequest(http.MethodPost, "http://upstream.test/start", body)

	resp, err := newRedirectFollowingTransport(transport, 3, RedirectDecisionFollow, "").RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	require.False(t, followed, "a failed upload must not be replayed as a truncated request")
}
