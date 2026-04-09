// Package metrics
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	requestCounter    *prometheus.CounterVec
	durationHistogram *prometheus.HistogramVec
	IngestedCounter   prometheus.Counter
}

func NewMetrics(r prometheus.Registerer) *Metrics {
	m := &Metrics{
		requestCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests by method, path and status",
			},
			[]string{"method", "path", "status"},
		),
		durationHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "http_request_duration_seconds",
				Help: "Duration of HTTP requests in seconds by method, path and status",
			},
			[]string{"method", "path", "status"},
		),
		IngestedCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "metrics_ingested_total",
				Help: "Total number of metrics ingested",
			},
		),
	}
	r.MustRegister(m.requestCounter, m.durationHistogram, m.IngestedCounter)
	return m
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		method := r.Method

		wrappedResp := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(wrappedResp, r)

		path := chi.RouteContext(r.Context()).RoutePattern()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(wrappedResp.Status())

		m.requestCounter.WithLabelValues(method, path, status).Inc()
		m.durationHistogram.WithLabelValues(method, path, status).Observe(duration)
	})
}
