package metrics

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricUpstreamRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pavonis",
		Subsystem: "upstream",
		Name:      "http_request_total",
		Help:      "Upstream round trips (hop starts at 0, empty code means no response).",
	}, []string{"site", "method", "code", "hop", "proxy"})

	metricUpstreamRequestElapsedSeconds = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pavonis",
		Subsystem: "upstream",
		Name:      "http_request_elapsed_seconds_total",
		Help:      "Cumulative round-trip duration, including body transfer.",
	}, []string{"site", "method", "code"})

	metricUpstreamRequestBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pavonis",
		Subsystem: "upstream",
		Name:      "http_request_bytes_total",
		Help:      "Total upstream request body bytes read.",
	}, []string{"site", "method", "code", "proxy"})

	metricUpstreamResponseBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pavonis",
		Subsystem: "upstream",
		Name:      "http_response_bytes_total",
		Help:      "Total upstream response body bytes read.",
	}, []string{"site", "method", "code", "proxy"})

	metricUpstreamRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pavonis",
		Subsystem: "upstream",
		Name:      "http_request_duration_seconds",
		Help:      "Round-trip duration, including body transfer.",
		Buckets:   httpRequestDurationBuckets,
	}, []string{"site"})

	metricUpstreamRequestSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pavonis",
		Subsystem: "upstream",
		Name:      "http_request_size_bytes",
		Help:      "Request body bytes read per round trip.",
		Buckets:   httpBodySizeBuckets,
	}, []string{"site"})

	metricUpstreamResponseSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pavonis",
		Subsystem: "upstream",
		Name:      "http_response_size_bytes",
		Help:      "Response body bytes read per round trip.",
		Buckets:   httpBodySizeBuckets,
	}, []string{"site"})

	metricUpstreamRequestsInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pavonis",
		Subsystem: "upstream",
		Name:      "http_requests_in_flight",
		Help:      "Upstream round trips in progress.",
	}, []string{"site", "method"})
)

type redirectHopKey struct{}

// WithRedirectHop carries the hop index of an upstream request through RoundTrip.
func WithRedirectHop(ctx context.Context, hop int) context.Context {
	return context.WithValue(ctx, redirectHopKey{}, hop)
}

type upstreamMetricsTransport struct {
	transport http.RoundTripper
	site      string
}

var _ http.RoundTripper = &upstreamMetricsTransport{}

// WrapUpstreamTransport records upstream HTTP metrics for each round trip.
func WrapUpstreamTransport(transport http.RoundTripper, site string) http.RoundTripper {
	return &upstreamMetricsTransport{transport: transport, site: site}
}

func (t *upstreamMetricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	method := req.Method
	redirectHop, _ := req.Context().Value(redirectHopKey{}).(int)
	inFlight := metricUpstreamRequestsInFlight.WithLabelValues(t.site, method)
	inFlight.Inc()

	var requestBytes atomic.Int64
	var code, currentProxyID string
	finish := func(responseBytes int64) {
		duration := time.Since(start).Seconds()
		inFlight.Dec()
		requestBodyBytes := float64(requestBytes.Load())
		responseBodyBytes := float64(responseBytes)

		metricUpstreamRequests.WithLabelValues(t.site, method, code, strconv.Itoa(redirectHop), currentProxyID).Inc()
		metricUpstreamRequestElapsedSeconds.WithLabelValues(t.site, method, code).Add(duration)
		metricUpstreamRequestBytes.WithLabelValues(t.site, method, code, currentProxyID).Add(requestBodyBytes)
		metricUpstreamResponseBytes.WithLabelValues(t.site, method, code, currentProxyID).Add(responseBodyBytes)

		metricUpstreamRequestDuration.WithLabelValues(t.site).Observe(duration)
		metricUpstreamRequestSize.WithLabelValues(t.site).Observe(requestBodyBytes)
		metricUpstreamResponseSize.WithLabelValues(t.site).Observe(responseBodyBytes)
	}
	bodyPending := false
	defer func() {
		if !bodyPending {
			finish(0)
		}
	}()

	out := req.WithContext(utils.WithProxyAttemptHook(req.Context(), func(id string) { currentProxyID = id }))
	if req.Body != nil && req.Body != http.NoBody {
		out.Body = &countingRequestBody{ReadCloser: req.Body, bytesRead: &requestBytes}
		if getBody := req.GetBody; getBody != nil {
			out.GetBody = func() (io.ReadCloser, error) {
				body, err := getBody()
				if err != nil || body == nil || body == http.NoBody {
					return body, err
				}
				return &countingRequestBody{ReadCloser: body, bytesRead: &requestBytes}, nil
			}
		}
	}

	resp, err := t.transport.RoundTrip(out)
	if err != nil {
		return resp, err
	}
	code = strconv.Itoa(resp.StatusCode)
	if resp.Body != nil && resp.Body != http.NoBody {
		resp.Body = &measuredUpstreamResponseBody{ReadCloser: resp.Body, finish: finish}
		bodyPending = true
	}
	return resp, nil
}

type measuredUpstreamResponseBody struct {
	io.ReadCloser
	bytesRead atomic.Int64
	once      sync.Once
	finish    func(int64)
}

var _ io.ReadCloser = &measuredUpstreamResponseBody{}

func (b *measuredUpstreamResponseBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.bytesRead.Add(int64(n))
	if err == io.EOF {
		b.observe()
	}
	return n, err
}

func (b *measuredUpstreamResponseBody) Close() error {
	defer b.observe()
	return b.ReadCloser.Close()
}

func (b *measuredUpstreamResponseBody) observe() {
	b.once.Do(func() { b.finish(b.bytesRead.Load()) })
}
