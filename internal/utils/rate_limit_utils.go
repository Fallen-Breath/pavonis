package utils

import (
	"golang.org/x/time/rate"
	"math"
)

func constrainToInt(f float64) int {
	if f < 0 {
		return 0
	}
	if f > float64(math.MaxInt) {
		return math.MaxInt
	}
	return int(f)
}

const bytePerMib = 1024 * 1024

func CreateTrafficRateLimiter(avgMbps, burstMb, maxMbps *float64) RateLimiter {
	limiter := NewMultiRateLimiter()
	if avgMbps != nil {
		if burstMb == nil {
			burstMb = ToPtr(math.Min(*avgMbps, 1.0))
		}
		limiter.AddLimiter(rate.NewLimiter(rate.Limit(*avgMbps*bytePerMib), constrainToInt(*burstMb*bytePerMib)))
	}
	if maxMbps != nil {
		limiter.AddLimiter(rate.NewLimiter(rate.Limit(*maxMbps*bytePerMib), constrainToInt(*maxMbps*bytePerMib)))
	}
	return limiter
}

func CreateRequestRateLimiter(qps, qpm, qph *float64) RateLimiter {
	limiter := NewMultiRateLimiter()
	if qps != nil {
		limiter.AddLimiter(rate.NewLimiter(rate.Limit(*qps), constrainToInt(*qps)))
	}
	if qpm != nil {
		limiter.AddLimiter(rate.NewLimiter(rate.Limit(*qpm/60), constrainToInt(*qpm)))
	}
	if qph != nil {
		limiter.AddLimiter(rate.NewLimiter(rate.Limit(*qph/60/60), constrainToInt(*qph)))
	}
	return limiter
}
