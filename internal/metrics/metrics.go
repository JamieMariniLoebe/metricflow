// Package metrics
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	requestCounter          *prometheus.CounterVec
	durationHistogram       *prometheus.HistogramVec
	IngestedCounter         prometheus.Counter
	IngestQueueDepth        prometheus.Gauge
	IngestRequestsShedTotal prometheus.Counter
}

func NewMetrics(r prometheus.Registerer, pool *pgxpool.Pool) *Metrics {
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
				Name:    "http_request_duration_seconds",
				Help:    "Duration of HTTP requests in seconds by method, path and status",
				Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
			},
			[]string{"method", "path", "status"},
		),
		IngestedCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "metrics_ingested_total",
				Help: "Total number of metrics ingested",
			},
		),
		IngestQueueDepth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "ingest_queue_depth",
				Help: "Total number of items in channel",
			},
		),
		IngestRequestsShedTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "ingest_requests_shed_total",
				Help: "Total number of requests shed",
			},
		),
	}
	r.MustRegister(m.requestCounter, m.durationHistogram, m.IngestedCounter, m.IngestQueueDepth, m.IngestRequestsShedTotal)

	r.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "pgxpool_acquired_conns",
			Help: "Currently acquired connections",
		},
		func() float64 {
			return float64(pool.Stat().AcquiredConns())
		},
	))

	r.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "pgxpool_idle_conns",
			Help: "Currently idle connections",
		},
		func() float64 {
			return float64(pool.Stat().IdleConns())
		},
	))

	r.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "pgxpool_max_conns",
			Help: "Current maximum connections",
		},
		func() float64 {
			return float64(pool.Stat().MaxConns())
		},
	))

	r.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "pgxpool_acquire_wait_seconds",
			Help: "Total time spent waiting due to pool being empty",
		},
		func() float64 {
			return pool.Stat().EmptyAcquireWaitTime().Seconds()
		},
	))

	r.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "pgxpool_acquire_duration_seconds",
			Help: "Total time spent acquiring",
		},
		func() float64 {
			return pool.Stat().AcquireDuration().Seconds()
		},
	))

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
