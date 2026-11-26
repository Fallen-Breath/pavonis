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
		const peakBurstWindow = 50 * time.Millisecond
		bytesPerSecond := *maxMbps * bytePerMib
		visit(rate.Limit(bytesPerSecond), max(1, constrainToInt(bytesPerSecond*peakBurstWindow.Seconds())))
	}
}

func CalculateTrafficRateLimitRefillDuration(avgMbps, burstMb, maxMbps *float64) time.Duration {
	var duration time.Duration
	visitTrafficBuckets(avgMbps, burstMb, maxMbps, func(limit rate.Limit, burst int) {
		duration = max(duration, calculateRefillDuration(limit, burst))
	})
	return duration
}

func CreateRequestRateBucket(value float64, interval time.Duration) *rate.Limiter {
	return rate.NewLimiter(calculateRequestRateLimitParams(value, interval))
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
