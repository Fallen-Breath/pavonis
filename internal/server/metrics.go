package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricRequestServed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pavonis",
		Subsystem: "server",
		Name:      "http_request_total",
		Help:      "Total number of HTTP requests served",
	}, []string{"code"})
)
