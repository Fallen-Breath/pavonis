package metrics

import (
	"io"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type countingRequestBody struct {
	io.ReadCloser
	bytesRead *atomic.Int64
}

var _ io.ReadCloser = &countingRequestBody{}

func (b *countingRequestBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.bytesRead.Add(int64(n))
	return n, err
}

var (
	httpRequestDurationBuckets = []float64{0.01, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 15, 30, 60, 300, 900, 1800}
	httpBodySizeBuckets        = []float64{1 << 5, 1 << 10, 1 << 12, 1 << 14, 1 << 16, 1 << 18, 1 << 20, 1 << 22, 1 << 24, 1 << 26, 1 << 28, 1 << 30, 1 << 32}

	metricRequestServed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_request_total",
		Help:      "Total number of HTTP requests served",
	}, []string{"site", "method", "code"})

	metricRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request duration in seconds, including response body transfer",
		Buckets:   httpRequestDurationBuckets,
	}, []string{"site", "method", "code"})

	metricRequestSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_request_size_bytes",
		Help:      "HTTP request body size in bytes actually read",
		Buckets:   httpBodySizeBuckets,
	}, []string{"site", "method", "code"})

	metricResponseSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_response_size_bytes",
		Help:      "HTTP response body size in bytes actually written",
		Buckets:   httpBodySizeBuckets,
	}, []string{"site", "method", "code"})

	metricRequestsInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_requests_in_flight",
		Help:      "Number of HTTP requests currently being processed",
	}, []string{"site", "method"})

	metricRateLimitRejections = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "rate_limit_rejections_total",
		Help:      "Total number of requests rejected by the local request rate limiter",
	}, []string{"site"})
)

// TrackServerRequest counts a request as in flight until the caller records its outcome.
func TrackServerRequest(r *http.Request, site string) func(status int, durationSeconds float64, responseBytes int64, rateLimited bool) {
	var requestBytes atomic.Int64
	if r.Body != nil && r.Body != http.NoBody {
		r.Body = &countingRequestBody{ReadCloser: r.Body, bytesRead: &requestBytes}
	}
	method := r.Method
	inFlight := metricRequestsInFlight.WithLabelValues(site, method)
	inFlight.Inc()

	return func(status int, durationSeconds float64, responseBytes int64, rateLimited bool) {
		defer inFlight.Dec()
		code := strconv.Itoa(status)
		metricRequestServed.WithLabelValues(site, method, code).Inc()
		metricRequestDuration.WithLabelValues(site, method, code).Observe(durationSeconds)
		metricRequestSize.WithLabelValues(site, method, code).Observe(float64(requestBytes.Load()))
		metricResponseSize.WithLabelValues(site, method, code).Observe(float64(responseBytes))
		if rateLimited {
			metricRateLimitRejections.WithLabelValues(site).Inc()
		}
	}
}
