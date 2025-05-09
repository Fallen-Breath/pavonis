package common

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type TimeLimitedTransport struct {
	transport http.RoundTripper
	timeout   time.Duration
}

var _ http.RoundTripper = &TimeLimitedTransport{}

func NewTimeLimitedTransport(transport http.RoundTripper, timeout time.Duration) *TimeLimitedTransport {
	return &TimeLimitedTransport{
		transport: transport,
		timeout:   timeout,
	}
}

func (t *TimeLimitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(req.Context(), t.timeout)
	defer cancel()

	newReq := req.WithContext(ctx)

	resp, err := t.transport.RoundTrip(newReq)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, NewHttpError(http.StatusGatewayTimeout, "time limit exceeded")
		}
		return nil, err
	}
	return resp, nil
}
