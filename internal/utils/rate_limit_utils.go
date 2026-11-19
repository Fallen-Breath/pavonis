package utils

import (
	"golang.org/x/time/rate"
	"math"
	"time"
)

func constrainToInt(f float64) int {
	if f < 0 {
		return 0
	}
	if f >= float64(math.MaxInt) {
		return math.MaxInt
	}
	return int(f)
}

const bytePerMib = 1024 * 1024

func CreateTrafficRateLimiter(avgMbps, burstMb, maxMbps *float64) RateLimiter {
	limiter := NewMultiRateLimiter()
	visitTrafficBuckets(avgMbps, burstMb, maxMbps, func(limit rate.Limit, burst int) {
		limiter.AddLimiter(rate.NewLimiter(limit, burst))
	})
	return limiter
}

func visitTrafficBuckets(avgMbps, burstMb, maxMbps *float64, visit func(rate.Limit, int)) {
	if avgMbps != nil {
		burst := math.Min(*avgMbps, 0.125)
		if burstMb != nil {
			burst = *burstMb
		}
		visit(rate.Limit(*avgMbps*bytePerMib), max(1, constrainToInt(burst*bytePerMib)))
	}
	if maxMbps != nil {
		visit(rate.Limit(*maxMbps*bytePerMib), max(1, constrainToInt(*maxMbps*bytePerMib)))
	}
}

func CalculateTrafficRateLimitRefillDuration(avgMbps, burstMb, maxMbps *float64) time.Duration {
	var duration time.Duration
	visitTrafficBuckets(avgMbps, burstMb, maxMbps, func(limit rate.Limit, burst int) {
		duration = max(duration, calculateRefillDuration(limit, burst))
	})
	return duration
}

func CreateRequestRateLimiter(qps, qpm, qph *float64) RateLimiter {
	limiter := NewMultiRateLimiter()
	add := func(value *float64, interval time.Duration) {
		if value != nil {
			limit, burst := calculateRequestRateLimitParams(*value, interval)
			limiter.AddLimiter(rate.NewLimiter(limit, burst))
		}
	}
	add(qps, time.Second)
	add(qpm, time.Minute)
	add(qph, time.Hour)
	return limiter
}

func calculateRequestRateLimitParams(value float64, interval time.Duration) (rate.Limit, int) {
	return rate.Limit(value / interval.Seconds()), max(1, constrainToInt(value))
}

// CalculateRequestRateLimitRefillDuration returns the time needed to refill an empty
// request bucket. Unset limits have no refill time; values must be validated.
func CalculateRequestRateLimitRefillDuration(value *float64, interval time.Duration) time.Duration {
	if value == nil {
		return 0
	}
	return calculateRefillDuration(calculateRequestRateLimitParams(*value, interval))
}

func calculateRefillDuration(limit rate.Limit, burst int) time.Duration {
	nanos := math.Ceil(float64(burst) / float64(limit) * float64(time.Second))
	if nanos >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(nanos)
}
