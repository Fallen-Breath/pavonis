package common

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"

	appctx "github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/stretchr/testify/require"
)

func TestRedirectFollowingTransportRequestBehavior(t *testing.T) {
	const payload = "request payload\x00with binary bytes\xff"
	type request struct{ method, body string }
	tests := []struct {
		name  string
		codes []int
		want  []request
	}{
		{"POST 301", []int{301}, []request{{"POST", payload}, {"GET", ""}}},
		{"POST 302", []int{302}, []request{{"POST", payload}, {"GET", ""}}},
		{"PUT 301", []int{301}, []request{{"PUT", payload}, {"PUT", payload}}},
		{"PUT 302", []int{302}, []request{{"PUT", payload}, {"PUT", payload}}},
		{"POST 303", []int{303}, []request{{"POST", payload}, {"GET", ""}}},
		{"PUT 303", []int{303}, []request{{"PUT", payload}, {"GET", ""}}},
		{"HEAD 303", []int{303}, []request{{"HEAD", ""}, {"HEAD", ""}}},
		{"POST 307", []int{307}, []request{{"POST", payload}, {"POST", payload}}},
		{"POST 308", []int{308}, []request{{"POST", payload}, {"POST", payload}}},
		{"POST 307 then 307", []int{307, 307}, []request{{"POST", payload}, {"POST", payload}, {"POST", payload}}},
		{"POST 307 then 303", []int{307, 303}, []request{{"POST", payload}, {"POST", payload}, {"GET", ""}}},
		{"POST 303 then 307", []int{303, 307}, []request{{"POST", payload}, {"GET", ""}, {"GET", ""}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var received []request
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var body []byte
				if req.Body != nil {
					defer req.Body.Close()
					var err error
					body, err = io.ReadAll(req.Body)
					if err != nil {
						return nil, err
					}
				}
				step := len(received)
				received = append(received, request{req.Method, string(body)})
				resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody, Request: req}
				if step < len(tc.codes) {
					resp.StatusCode = tc.codes[step]
					resp.Header.Set("Location", "/next")
				}
				return resp, nil
			})
			req := httptest.NewRequest(tc.want[0].method, "http://upstream.test/start", strings.NewReader(tc.want[0].body))
			tr := NewRedirectFollowingTransport(appctx.NewRequestContext("test", "127.0.0.1"), transport, 3,
				func(*http.Response) *RedirectResult { return &RedirectResult{Decision: RedirectDecisionFollow} }, nil)

			resp, err := tr.RoundTrip(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, tc.want, received)
		})
	}
}

func TestRunReverseProxyRedirectBodyHandling(t *testing.T) {
	const payload = "request payload"
	for _, tc := range []struct {
		name          string
		codes         []int
		body          string
		unknownLength bool
		dropBody      bool
	}{
		{"known length replay", []int{307}, payload, false, false},
		{"unknown length repeated replay", []int{307, 307}, payload, true, false},
		{"dropped body stays dropped", []int{303, 307}, payload, true, true},
		{"empty body retains content headers", []int{307}, "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type receivedRequest struct {
				method, body  string
				header        http.Header
				contentLength int64
				err           error
			}
			received := make(chan receivedRequest, len(tc.codes)+1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				step, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
				if err != nil || step < 0 || step > len(tc.codes) {
					http.Error(w, "unexpected redirect target", http.StatusBadRequest)
					return
				}
				body, err := io.ReadAll(r.Body)
				received <- receivedRequest{r.Method, string(body), r.Header.Clone(), r.ContentLength, err}
				if step < len(tc.codes) {
					w.Header().Set("Location", "/"+strconv.Itoa(step+1))
					w.WriteHeader(tc.codes[step])
				}
			}))
			defer upstream.Close()

			var body io.Reader = strings.NewReader(tc.body)
			if tc.unknownLength {
				body = onlyReader{body}
			}
			req := httptest.NewRequest(http.MethodPost, "/", body)
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("Content-Encoding", "identity")
			rec := httptest.NewRecorder()
			helper := newTestRequestHelper(t)
			helper.RunReverseProxy(appctx.NewRequestContext("test", "127.0.0.1"), rec, req, mustURL(t, upstream.URL+"/0"))

			require.Equal(t, http.StatusOK, rec.Code)
			require.Len(t, received, len(tc.codes)+1)
			for i := 0; i <= len(tc.codes); i++ {
				got := <-received
				require.NoError(t, got.err, "hop %d", i)
				if tc.dropBody && i > 0 {
					require.Equal(t, http.MethodGet, got.method, "hop %d", i)
					require.Empty(t, got.body, "hop %d", i)
					require.Empty(t, got.header.Get("Content-Type"), "hop %d", i)
					require.Empty(t, got.header.Get("Content-Encoding"), "hop %d", i)
				} else {
					require.Equal(t, http.MethodPost, got.method, "hop %d", i)
					require.Equal(t, tc.body, got.body, "hop %d", i)
					require.Equal(t, "application/octet-stream", got.header.Get("Content-Type"), "hop %d", i)
					require.Equal(t, "identity", got.header.Get("Content-Encoding"), "hop %d", i)
				}
				if got.contentLength >= 0 {
					require.Equal(t, int64(len(got.body)), got.contentLength, "hop %d body length", i)
				}
			}
		})
	}
}

func TestRedirectFollowingTransportRejectsFailedBodyReplay(t *testing.T) {
	followed := false
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Body != nil {
			defer req.Body.Close()
		}
		if req.URL.Path == "/final" {
			followed = true
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
		}
		// The upstream redirects even though reading the upload has failed.
		_, _ = io.Copy(io.Discard, req.Body)
		return &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Header:     http.Header{"Location": []string{"/final"}},
			Body:       http.NoBody,
			Request:    req,
		}, nil
	})
	body := io.MultiReader(strings.NewReader("partial payload"), iotest.ErrReader(io.ErrUnexpectedEOF))
	req := httptest.NewRequest(http.MethodPost, "http://upstream.test/start", body)
	tr := NewRedirectFollowingTransport(appctx.NewRequestContext("test", "127.0.0.1"), transport, 3,
		func(*http.Response) *RedirectResult { return &RedirectResult{Decision: RedirectDecisionFollow} }, nil)

	resp, err := tr.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	require.False(t, followed, "a failed upload must not be replayed as a truncated request")
}
