package limits

import (
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"golang.org/x/time/rate"
)

// quota is one budget of concurrent requests, request rate and traffic
type quota struct {
	active      int
	concurrency *int
	requests    []requestBucket
	traffic     utils.RateLimiter // nil means unlimited
}

type requestBucket struct {
	name string
	*rate.Limiter
}

type requestRate struct {
	name     string
	value    *float64
	interval time.Duration
}

func getRequestRates(cfg *config.ClientQuotaConfig) [3]requestRate {
	return [3]requestRate{
		{"request_per_second", cfg.RequestPerSecond, time.Second},
		{"request_per_minute", cfg.RequestPerMinute, time.Minute},
		{"request_per_hour", cfg.RequestPerHour, time.Hour},
	}
}

func newQuota(cfg *config.ClientQuotaConfig) *quota {
	q := &quota{concurrency: cfg.RequestConcurrency}
	for _, r := range getRequestRates(cfg) {
		if r.value != nil {
			q.requests = append(q.requests, requestBucket{r.name, utils.CreateRequestRateBucket(*r.value, r.interval)})
		}
	}
	if cfg.TrafficAvgMibps != nil || cfg.TrafficMaxMibps != nil {
		q.traffic = utils.CreateTrafficRateLimiter(cfg.TrafficAvgMibps, cfg.TrafficBurstMib, cfg.TrafficMaxMibps)
	}
	return q
}

func (q *quota) findExceededLimit(now time.Time) string {
	if q.concurrency != nil && q.active >= *q.concurrency {
		return "request_concurrency"
	}
	for _, bucket := range q.requests {
		if bucket.TokensAt(now) < 1 {
			return bucket.name
		}
	}
	return ""
}

func (q *quota) startRequest(now time.Time) {
	q.active++
	for _, bucket := range q.requests {
		bucket.AllowN(now, 1)
	}
}

func (q *quota) finishRequest() {
	q.active--
}

// scope holds the limits of the global config or of a quota group
type scope struct {
	group            string
	allClientsConfig *config.ClientQuotaConfig
	allClients       *quota                    // nil means unlimited
	perClientConfig  *config.ClientQuotaConfig // nil means unlimited
	idleTTL          time.Duration             // how long an idle per-client quota takes to refill
}

// newScope returns nil if cfg limits nothing
func newScope(group string, cfg *config.ResourceQuotaConfig) *scope {
	s := &scope{group: group}
	if cfg.AllClients != (config.ClientQuotaConfig{}) {
		s.allClientsConfig = &cfg.AllClients
		s.allClients = newQuota(s.allClientsConfig)
	}
	if c := &cfg.PerClient; *c != (config.ClientQuotaConfig{}) {
		s.perClientConfig = c
		s.idleTTL = utils.CalculateTrafficRateLimitRefillDuration(c.TrafficAvgMibps, c.TrafficBurstMib, c.TrafficMaxMibps)
		for _, r := range getRequestRates(c) {
			s.idleTTL = max(s.idleTTL, utils.CalculateRequestRateLimitRefillDuration(r.value, r.interval))
		}
	}
	if s.allClients == nil && s.perClientConfig == nil {
		return nil
	}
	return s
}
