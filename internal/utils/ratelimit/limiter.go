package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
)

type Limiter interface {
	Burst() int
	WaitN(ctx context.Context, n int) (err error)
}

var _ Limiter = &rate.Limiter{}
