package common

import (
	"errors"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/Fallen-Breath/pavonis/internal/utils/ioutils"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

type RedirectDecision string

const (
	RedirectDecisionFollow  = "follow"
	RedirectDecisionRewrite = "rewrite"
	RedirectDecisionReturn  = "return"
	RedirectDecisionAbort   = "abort"
)

type RedirectResult struct {
	Decision RedirectDecision
	Value    string
	Err      error
}

type RedirectHandler = func(resp *http.Response) *RedirectResult

type RedirectFollowingTransport struct {
	ctx             *context.RequestContext
	transport       http.RoundTripper
	maxRedirects    int
	redirectHandler RedirectHandler
	requestRecorder func(*http.Request)
}

var _ http.RoundTripper = &RedirectFollowingTransport{}

func NewRedirectFollowingTransport(ctx *context.RequestContext, transport http.RoundTripper, maxRedirect int, redirectHandler RedirectHandler, requestRecorder func(*http.Request)) *RedirectFollowingTransport {
	if redirectHandler == nil {
		panic(errors.New("redirect handler must not be nil"))
	}
	return &RedirectFollowingTransport{
		ctx:             ctx,
		transport:       transport,
		maxRedirects:    maxRedirect,
		redirectHandler: redirectHandler,
		requestRecorder: requestRecorder,
	}
}

func IsStatusCodeRedirect(code int) bool {
	return code == http.StatusMovedPermanently ||
		code == http.StatusFound ||
		code == http.StatusSeeOther ||
		code == http.StatusTemporaryRedirect ||
		code == http.StatusPermanentRedirect
}

// determineRedirectBehavior determines the next request after a redirect is
// selected for following:
//   - 301/302 with POST: use GET and drop the body.
//   - 301/302 with any other method: preserve the method and body.
//   - 303: use GET (keep HEAD as HEAD) and drop the body.
//   - 307/308: preserve the method and body.
//
// Each hop uses the current request, so a dropped body stays dropped on later
// redirects. Preserving a body requires complete replay; otherwise following fails.
// Whether to follow, rewrite, return, or reject a redirect is handled separately
// by RoundTrip and redirectHandler, together with the redirect limit.
func determineRedirectBehavior(method string, code int) (nextMethod string, keepBody bool) {
	switch code {
	case http.StatusMovedPermanently, http.StatusFound:
		if method == http.MethodPost {
			return http.MethodGet, false
		}
	case http.StatusSeeOther:
		if method == http.MethodHead {
			return http.MethodHead, false
		}
		return http.MethodGet, false
	}
	return method, true
}

func (t *RedirectFollowingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := t.transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	req = req.Clone(req.Context())

	if req.Body != nil && req.Body != http.NoBody {
		contentLength := req.ContentLength
		if contentLength == 0 {
			// For a non-nil request body, zero means the length is unknown.
			contentLength = -1
		}
		bufferedBody := ioutils.NewBufferedReusableReaderWithContentLength(req.Body, 8192, contentLength)
		req.Body = bufferedBody
		req.GetBody = func() (io.ReadCloser, error) {
			reader, ok := bufferedBody.GetNextReader()
			if !ok {
				return nil, errors.New("cannot replay request body")
			}
			return reader, nil
		}
	} else {
		req.Body = nil
		req.GetBody = nil
	}

	redirectCount := 0
	for {
		if t.requestRecorder != nil {
			t.requestRecorder(req)
		}

		resp, err := transport.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		if !IsStatusCodeRedirect(resp.StatusCode) {
			return resp, nil
		}
		if redirectCount >= t.maxRedirects {
			return resp, nil
		}

		rr := t.redirectHandler(resp)
		if rr == nil {
			panic(errors.New("redirect result must not be nil"))
		}
		switch rr.Decision {
		case RedirectDecisionFollow:
			// pass
		case RedirectDecisionRewrite:
			resp.Header.Set("Location", rr.Value)
			return resp, nil
		case RedirectDecisionReturn:
			return resp, nil
		case RedirectDecisionAbort:
			_ = resp.Body.Close()
			return nil, rr.Err
		}

		location, err := resp.Location()
		if err != nil {
			if log.IsLevelEnabled(log.DebugLevel) {
				log.Debugf("%sFailed to get the Location header for a redirect response, do not follow: %+v", t.ctx.LogPrefix, utils.MaskResponseForLogging(resp))
			}
			return resp, nil
		}

		_ = resp.Body.Close()

		log.Debugf("%sFollowing redirect (%s) from %+q to %+q (%s): %+v", t.ctx.LogPrefix, resp.Status, req.URL, location, resp.Status, utils.MaskResponseForLogging(resp))
		oldReq := req
		nextMethod, keepBody := determineRedirectBehavior(oldReq.Method, resp.StatusCode)
		var body io.ReadCloser
		if keepBody && oldReq.GetBody != nil {
			body, err = oldReq.GetBody()
			if err != nil {
				return nil, err
			}
		}
		req, err = http.NewRequestWithContext(oldReq.Context(), nextMethod, location.String(), body)
		if err != nil {
			if body != nil {
				_ = body.Close()
			}
			return nil, err
		}

		if keepBody && body != nil {
			req.GetBody = oldReq.GetBody
			req.ContentLength = oldReq.ContentLength
			req.TransferEncoding = append([]string(nil), oldReq.TransferEncoding...)
			req.Trailer = oldReq.Trailer.Clone()
		}
		rebuildReqHeader(req, oldReq, keepBody)

		redirectCount++
	}
}

func rebuildReqHeader(req *http.Request, oldReq *http.Request, keepBody bool) {
	// remove sensitive auth header if host mismatch
	stripSensitiveHeaders := req.URL.Hostname() != oldReq.URL.Hostname()

	req.Header = oldReq.Header.Clone()
	if stripSensitiveHeaders {
		for _, key := range []string{"Authorization", "Www-Authenticate", "Cookie", "Cookie2"} {
			req.Header.Del(key)
		}
	}
	if !keepBody {
		for _, key := range []string{
			"Content-Length", "Content-Type", "Content-Encoding", "Content-Language",
			"Content-Location", "Content-Range", "Content-Md5", "Digest", "Content-Digest",
			"Repr-Digest", "Last-Modified", "Transfer-Encoding", "Trailer", "Expect",
		} {
			req.Header.Del(key)
		}
	}
}
