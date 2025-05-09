package common

import (
	"context"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"io"
	"net/http"
)

type RateLimitedTransport struct {
	transport http.RoundTripper
	limiter   utils.TransportRateLimiter
}

var _ http.RoundTripper = &RateLimitedTransport{}

func NewRateLimitedTransport(transport http.RoundTripper, limiter utils.TransportRateLimiter) *RateLimitedTransport {
	return &RateLimitedTransport{
		transport: transport,
		limiter:   limiter,
	}
}

func (t *RateLimitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		req.Body = &rateLimitedReader{
			reader:  req.Body,
			limiter: t.limiter,
			ctx:     req.Context(),
		}
	}

	resp, err := t.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.Body != nil {
		resp.Body = &rateLimitedReader{
			reader:  resp.Body,
			limiter: t.limiter,
			ctx:     req.Context(),
		}
	}

	return resp, nil
}

type rateLimitedReader struct {
	reader  io.Reader
	limiter utils.TransportRateLimiter
	ctx     context.Context
}

func (r *rateLimitedReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err != nil {
		return n, err
	}

	if n > 0 {
		if err := r.limiter.WaitN(r.ctx, n); err != nil {
			return n, err
		}
	}

	return n, nil
}

func (r *rateLimitedReader) Close() error {
	if closer, ok := r.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
