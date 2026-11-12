package common

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	appctx "github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// onlyReader hides other interfaces, so the http client cannot know the body size and sends it chunked
type onlyReader struct{ io.Reader }

func newTestRequestHelper(t *testing.T) *RequestHelper {
	t.Helper()
	mode := config.SiteModeSpeedTest
	cfg := &config.Config{Sites: []*config.SiteConfig{{Mode: &mode, Host: config.SiteHosts{"*"}}}}
	require.NoError(t, cfg.Init())
	factory, err := NewRequestHelperFactory(cfg)
	require.NoError(t, err)
	t.Cleanup(factory.Shutdown)
	return factory.NewRequestHelper("test", nil)
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestRunReverseProxyAppliesHeaderModifications(t *testing.T) {
	var gotHeader http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	helper := newTestRequestHelper(t)
	helper.cfg.Request.Header.Modify = &map[string]string{"X-Test": "rewritten"}
	helper.cfg.Request.Header.Delete = &[]string{"X-Remove"}
	helper.cfg.Response.Header.Modify = &map[string]string{"X-Response": "rewritten"}
	helper.cfg.Response.Header.Delete = &[]string{"X-Upstream"}

	req := httptest.NewRequest(http.MethodGet, "/source", nil)
	req.Header.Set("X-Test", "original")
	req.Header.Set("X-Remove", "present")
	rec := httptest.NewRecorder()
	helper.RunReverseProxy(appctx.NewRequestContext("test", "127.0.0.1"), rec, req, mustURL(t, upstream.URL))

	require.Equal(t, "rewritten", gotHeader.Get("X-Test"))
	require.Empty(t, gotHeader.Get("X-Remove"))
	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "rewritten", rec.Header().Get("X-Response"))
	require.Empty(t, rec.Header().Get("X-Upstream"))
	require.Equal(t, "ok", rec.Body.String())
}

func TestRunReverseProxyReplaysBodyOnRedirect(t *testing.T) {
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
			newTestRequestHelper(t).RunReverseProxy(appctx.NewRequestContext("test", "127.0.0.1"), rec, req, mustURL(t, upstream.URL+"/0"))

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

func TestCreateErrorHandlerMapsErrors(t *testing.T) {
	h := newTestRequestHelper(t).createErrorHandler(appctx.NewRequestContext("test", "127.0.0.1"))
	for _, tc := range []struct {
		name string
		err  error
		code int
	}{
		{"timeout", context.DeadlineExceeded, http.StatusGatewayTimeout},
		{"http error", NewHttpError(http.StatusForbidden, "blocked"), http.StatusForbidden},
		{"unknown", errors.New("upstream failed"), http.StatusBadGateway},
	} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, "/", nil), tc.err)
		require.Equal(t, tc.code, rec.Code, tc.name)
	}
}

func TestFetchHeaderReturnsUpstreamStatusAndHeaderWithoutFollowingRedirects(t *testing.T) {
	var gotMethod, gotAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotAuthorization = r.Method, r.Header.Get("Authorization")
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/v2/", http.StatusFound)
			return
		}
		w.Header().Set("Www-Authenticate", `Bearer realm="https://auth.test/token"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	helper := newTestRequestHelper(t)
	ctx := appctx.NewRequestContext("test", "127.0.0.1")
	clientReq := httptest.NewRequest(http.MethodPost, "/source", strings.NewReader("body"))
	clientReq.Header.Set("Authorization", "Bearer client")

	status, header, err := helper.FetchHeader(ctx, clientReq, mustURL(t, upstream.URL+"/v2/"))
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, status)
	require.Equal(t, `Bearer realm="https://auth.test/token"`, header.Get("Www-Authenticate"))
	require.Equal(t, http.MethodGet, gotMethod)
	require.Empty(t, gotAuthorization)

	status, _, err = helper.FetchHeader(ctx, clientReq, mustURL(t, upstream.URL+"/redirect"))
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, status)
}
