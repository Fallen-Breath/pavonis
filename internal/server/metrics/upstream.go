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
		Help:      "Total number of upstream HTTP requests by zero-based redirect hop and last attempted request proxy (empty for direct or no attempt), including requests without an HTTP response (empty code)",
	}, []string{"site", "method", "code", "hop", "proxy"})

	metricUpstreamRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pavonis",
		Subsystem: "upstream",
		Name:      "http_request_duration_seconds",
		Help:      "Upstream HTTP request duration in seconds, including response body transfer",
		Buckets:   httpRequestDurationBuckets,
	}, []string{"site", "method", "code"})

	metricUpstreamRequestSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pavonis",
		Subsystem: "upstream",
		Name:      "http_request_size_bytes",
		Help:      "Upstream HTTP request body size in bytes actually read by the transport",
		Buckets:   httpBodySizeBuckets,
	}, []string{"site", "method", "code"})

	metricUpstreamResponseSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pavonis",
		Subsystem: "upstream",
		Name:      "http_response_size_bytes",
		Help:      "Upstream HTTP response body size in bytes actually read",
		Buckets:   httpBodySizeBuckets,
	}, []string{"site", "method", "code"})

	metricUpstreamRequestsInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pavonis",
		Subsystem: "upstream",
		Name:      "http_requests_in_flight",
		Help:      "Number of upstream HTTP requests currently in progress",
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
		metricUpstreamRequests.WithLabelValues(t.site, method, code, strconv.Itoa(redirectHop), currentProxyID).Inc()
		metricUpstreamRequestDuration.WithLabelValues(t.site, method, code).Observe(duration)
		metricUpstreamRequestSize.WithLabelValues(t.site, method, code).Observe(float64(requestBytes.Load()))
		metricUpstreamResponseSize.WithLabelValues(t.site, method, code).Observe(float64(responseBytes))
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
