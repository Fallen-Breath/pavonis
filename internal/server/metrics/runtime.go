package metrics

import "github.com/prometheus/client_golang/prometheus"

var requestProxyBlocked = prometheus.NewDesc(
	"pavonis_request_proxy_blocked",
	"Whether the request proxy is currently blocked (1) or unblocked (0)",
	[]string{"proxy"}, nil,
)

type runtimeCollector struct {
	readRequestProxyBlockStates func() map[string]bool
}

var _ prometheus.Collector = &runtimeCollector{}

// RegisterRuntimeMetrics adds metrics that read live server state to the default registry.
func RegisterRuntimeMetrics(readRequestProxyBlockStates func() map[string]bool) (func(), error) {
	collector := &runtimeCollector{readRequestProxyBlockStates: readRequestProxyBlockStates}
	if err := prometheus.Register(collector); err != nil {
		return nil, err
	}
	return func() { prometheus.Unregister(collector) }, nil
}

func (c *runtimeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- requestProxyBlocked
}

func (c *runtimeCollector) Collect(ch chan<- prometheus.Metric) {
	for id, blocked := range c.readRequestProxyBlockStates() {
		value := 0.0
		if blocked {
			value = 1
		}
		ch <- prometheus.MustNewConstMetric(requestProxyBlocked, prometheus.GaugeValue, value, id)
	}
}
