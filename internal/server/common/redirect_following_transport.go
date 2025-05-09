package common

import (
	"errors"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"net/http"
)

type RedirectFollowingTransport struct {
	transport    http.RoundTripper
	maxRedirects int
}

var _ http.RoundTripper = &RedirectFollowingTransport{}

func NewRedirectFollowingTransport(transport http.RoundTripper, maxRedirect int) *RedirectFollowingTransport {
	return &RedirectFollowingTransport{
		transport:    transport,
		maxRedirects: maxRedirect,
	}
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

		if !(300 <= resp.StatusCode && resp.StatusCode < 400) {
			return resp, nil
		}

		if redirectCount >= t.maxRedirects {
			return resp, nil
		}

		location, err := resp.Location()
		if err != nil {
			return resp, err
		}

		_ = resp.Body.Close()

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
