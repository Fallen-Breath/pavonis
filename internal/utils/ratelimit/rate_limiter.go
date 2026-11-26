package ratelimit

import (
	"context"
	"golang.org/x/time/rate"
	"math"
)

type Limiter interface {
	Burst() int
	WaitN(ctx context.Context, n int) (err error)
}

var _ Limiter = &rate.Limiter{}

type MultiLimiter struct {
	limiters []Limiter
}

var _ Limiter = &MultiLimiter{}

func NewMultiLimiter(limiters ...Limiter) *MultiLimiter {
	ml := &MultiLimiter{}
	for _, limiter := range limiters {
		ml.AddLimiter(limiter)
	}
	return ml
}

func (ml *MultiLimiter) AddLimiter(limiter Limiter) {
	if multi, ok := limiter.(*MultiLimiter); ok {
		for _, subLimiter := range multi.limiters {
			ml.limiters = append(ml.limiters, subLimiter)
		}
	} else {
		ml.limiters = append(ml.limiters, limiter)
	}
}

func (ml *MultiLimiter) Burst() int {
	burst := math.MaxInt
	for _, limiter := range ml.limiters {
		if bucket, ok := limiter.(*rate.Limiter); ok && bucket.Limit() == rate.Inf {
			continue
		}
		burst = min(burst, limiter.Burst())
	}
	return burst
}

func (ml *MultiLimiter) WaitN(ctx context.Context, n int) (err error) {
	for _, limiter := range ml.limiters {
		if err := limiter.WaitN(ctx, n); err != nil {
			return err
		}
	}
	return nil
}
