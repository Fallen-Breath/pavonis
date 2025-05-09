package utils

import (
	"context"
	"golang.org/x/time/rate"
)

type TransportRateLimiter interface {
	Allow() bool
	WaitN(ctx context.Context, n int) (err error)
}

var _ TransportRateLimiter = &rate.Limiter{}

type MultiRateLimiter struct {
	limiters []TransportRateLimiter
}

var _ TransportRateLimiter = &MultiRateLimiter{}

func NewMultiRateLimiter(limiters ...TransportRateLimiter) *MultiRateLimiter {
	ml := &MultiRateLimiter{}
	for _, limiter := range limiters {
		ml.AddLimiter(limiter)
	}
	return ml
}

func (ml *MultiRateLimiter) AddLimiter(limiter TransportRateLimiter) {
	if multi, ok := limiter.(*MultiRateLimiter); ok {
		for _, subLimiter := range multi.limiters {
			ml.limiters = append(ml.limiters, subLimiter)
		}
	} else {
		ml.limiters = append(ml.limiters, limiter)
	}
}

func (ml *MultiRateLimiter) Allow() bool {
	allow := true
	for _, limiter := range ml.limiters {
		allow = allow && limiter.Allow()
	}
	return allow
}

func (ml *MultiRateLimiter) WaitN(ctx context.Context, n int) (err error) {
	for _, limiter := range ml.limiters {
		if err := limiter.WaitN(ctx, n); err != nil {
			return err
		}
	}
	return nil
}
