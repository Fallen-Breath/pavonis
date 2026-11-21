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
		Help:      "Requests served.",
	}, []string{"site", "method", "code"})

	metricRequestElapsedSeconds = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_request_elapsed_seconds_total",
		Help:      "Cumulative request duration, including body transfer.",
	}, []string{"site", "method", "code"})

	metricRequestBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_request_bytes_total",
		Help:      "Total request body bytes read.",
	}, []string{"site", "method", "code"})

	metricResponseBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_response_bytes_total",
		Help:      "Total response body bytes written.",
	}, []string{"site", "method", "code"})

	metricRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_request_duration_seconds",
		Help:      "Request duration, including body transfer.",
		Buckets:   httpRequestDurationBuckets,
	}, []string{"site"})

	metricRequestSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_request_size_bytes",
		Help:      "Request body bytes read per request.",
		Buckets:   httpBodySizeBuckets,
	}, []string{"site"})

	metricResponseSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_response_size_bytes",
		Help:      "Response body bytes written per request.",
		Buckets:   httpBodySizeBuckets,
	}, []string{"site"})

	metricRequestsInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_requests_in_flight",
		Help:      "Requests in progress.",
	}, []string{"site", "method"})

	metricRateLimitRejections = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "rate_limit_rejections_total",
		Help:      "Requests rejected by the local rate limiter.",
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
		requestBodyBytes := float64(requestBytes.Load())
		responseBodyBytes := float64(responseBytes)

		metricRequestServed.WithLabelValues(site, method, code).Inc()
		metricRequestElapsedSeconds.WithLabelValues(site, method, code).Add(durationSeconds)
		metricRequestBytes.WithLabelValues(site, method, code).Add(requestBodyBytes)
		metricResponseBytes.WithLabelValues(site, method, code).Add(responseBodyBytes)

		metricRequestDuration.WithLabelValues(site).Observe(durationSeconds)
		metricRequestSize.WithLabelValues(site).Observe(requestBodyBytes)
		metricResponseSize.WithLabelValues(site).Observe(responseBodyBytes)
		if rateLimited {
			metricRateLimitRejections.WithLabelValues(site).Inc()
		}
	}
}
