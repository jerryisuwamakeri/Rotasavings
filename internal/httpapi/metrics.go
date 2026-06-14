package httpapi

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// Metrics is a tiny, dependency-free metrics collector that exposes counters in
// Prometheus text exposition format. It covers the essentials (request counts by
// status class, in-flight, and total latency) without pulling client_golang.
type Metrics struct {
	total          atomic.Int64
	status2xx      atomic.Int64
	status3xx      atomic.Int64
	status4xx      atomic.Int64
	status5xx      atomic.Int64
	inFlight       atomic.Int64
	latencyMsTotal atomic.Int64
}

func NewMetrics() *Metrics { return &Metrics{} }

func (m *Metrics) observe(status int, latencyMs int64) {
	m.total.Add(1)
	m.latencyMsTotal.Add(latencyMs)
	switch {
	case status >= 500:
		m.status5xx.Add(1)
	case status >= 400:
		m.status4xx.Add(1)
	case status >= 300:
		m.status3xx.Add(1)
	default:
		m.status2xx.Add(1)
	}
}

// Handler serves the metrics in Prometheus text format.
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP rota_http_requests_total Total HTTP requests by status class.\n")
		fmt.Fprintf(w, "# TYPE rota_http_requests_total counter\n")
		fmt.Fprintf(w, "rota_http_requests_total{class=\"2xx\"} %d\n", m.status2xx.Load())
		fmt.Fprintf(w, "rota_http_requests_total{class=\"3xx\"} %d\n", m.status3xx.Load())
		fmt.Fprintf(w, "rota_http_requests_total{class=\"4xx\"} %d\n", m.status4xx.Load())
		fmt.Fprintf(w, "rota_http_requests_total{class=\"5xx\"} %d\n", m.status5xx.Load())
		fmt.Fprintf(w, "# HELP rota_http_requests Total HTTP requests.\n")
		fmt.Fprintf(w, "# TYPE rota_http_requests counter\n")
		fmt.Fprintf(w, "rota_http_requests %d\n", m.total.Load())
		fmt.Fprintf(w, "# HELP rota_http_in_flight In-flight HTTP requests.\n")
		fmt.Fprintf(w, "# TYPE rota_http_in_flight gauge\n")
		fmt.Fprintf(w, "rota_http_in_flight %d\n", m.inFlight.Load())
		fmt.Fprintf(w, "# HELP rota_http_request_duration_ms_sum Total request latency in milliseconds.\n")
		fmt.Fprintf(w, "# TYPE rota_http_request_duration_ms_sum counter\n")
		fmt.Fprintf(w, "rota_http_request_duration_ms_sum %d\n", m.latencyMsTotal.Load())
	}
}
