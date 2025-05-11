package common

import (
	"errors"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type RedirectFollowingTransport struct {
	ctx          *context.RequestContext
	transport    http.RoundTripper
	maxRedirects int
}

var _ http.RoundTripper = &RedirectFollowingTransport{}

func NewRedirectFollowingTransport(ctx *context.RequestContext, transport http.RoundTripper, maxRedirect int) *RedirectFollowingTransport {
	return &RedirectFollowingTransport{
		ctx:          ctx,
		transport:    transport,
		maxRedirects: maxRedirect,
	}
}

func isStatusCodeRedirect(code int) bool {
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

	var bufferedBody *utils.BufferedReadCloser
	if req.Body != nil {
		bufferedBody = utils.NewBufferedReadCloser(req.Body, 8192)
		req.Body = bufferedBody
	}

	redirectCount := 0
	for {
		resp, err := transport.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		if !isStatusCodeRedirect(resp.StatusCode) {
			return resp, nil
		}

		if redirectCount >= t.maxRedirects {
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

		log.Debugf("%sRedirecting to %+q", t.ctx.LogPrefix, location)
		req, err = http.NewRequest(req.Method, location.String(), nil)
		if err != nil {
			return nil, err
		}

		for k, v := range resp.Request.Header {
			req.Header[k] = v
		}

		redirectCount++
		if bufferedBody != nil && redirectCount == 1 {
			newReader, ok := bufferedBody.GetNextReadCloser()
			if !ok {
				return nil, errors.New("no reader available")
			}
			req.Body = newReader
		}
	}
}
