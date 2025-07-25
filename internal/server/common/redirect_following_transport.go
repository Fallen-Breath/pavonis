package common

import (
	"errors"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/Fallen-Breath/pavonis/internal/utils/ioutils"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type RedirectDecision string

const (
	RedirectDecisionFollow  = "follow"
	RedirectDecisionRewrite = "rewrite"
	RedirectDecisionReturn  = "return"
)

type RedirectResult struct {
	Decision RedirectDecision
	Value    string
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

func (t *RedirectFollowingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := t.transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	req = req.Clone(req.Context())

	var bufferedBody *ioutils.BufferedReusableReader
	if req.Body != nil {
		bufferedBody = ioutils.NewBufferedReusableReader(req.Body, 8192)
		req.Body = bufferedBody
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
		req, err = http.NewRequestWithContext(req.Context(), req.Method, location.String(), nil)
		if err != nil {
			return nil, err
		}

		rebuildReqHeader(req, oldReq)

		redirectCount++
		if bufferedBody != nil && redirectCount == 1 {
			newReader, ok := bufferedBody.GetNextReader()
			if !ok {
				return nil, errors.New("no reader available")
			}
			req.Body = newReader
		}
	}
}

func rebuildReqHeader(req *http.Request, oldReq *http.Request) {
	// remove sensitive auth header if host mismatch
	stripSensitiveHeaders := req.URL.Hostname() != oldReq.URL.Hostname()

	for k, v := range oldReq.Header {
		sensitive := false
		switch k {
		case "Authorization", "Www-Authenticate", "Cookie", "Cookie2":
			sensitive = true
		}
		if !(sensitive && stripSensitiveHeaders) {
			req.Header[k] = v
		}
	}
}
