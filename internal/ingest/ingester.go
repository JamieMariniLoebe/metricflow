// Package ingest handles the Async Ingestion Pipeline
package ingest

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JamieMariniLoebe/metricflow/internal/models"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricInserter interface {
	InsertMetric(ctx context.Context, m models.Metric) error
}

type Ingester struct {
	ingest              chan models.Metric
	db                  MetricInserter
	workers             int
	wait                *sync.WaitGroup
	queueDepthGauge     prometheus.Gauge
	shedCounter         prometheus.Counter
	persistedCounter    prometheus.Counter
	workerPanicsCounter prometheus.Counter
	ctx                 context.Context
	cancel              context.CancelFunc
	closed              atomic.Bool
}

var ErrIngesterClosed = errors.New("ingester closed")
var ErrQueueFull = errors.New("queue full")

func NewIngester(s MetricInserter, w int, queueDepthGauge prometheus.Gauge, shedCounter prometheus.Counter, persistedCounter prometheus.Counter, workerPanicsCounter prometheus.Counter) *Ingester {
	i := make(chan models.Metric, w*5)
	ctx, cancel := context.WithCancel(context.Background())

	return &Ingester{
		ingest:              i,
		db:                  s,
		workers:             w,
		wait:                &sync.WaitGroup{},
		queueDepthGauge:     queueDepthGauge,
		shedCounter:         shedCounter,
		persistedCounter:    persistedCounter,
		workerPanicsCounter: workerPanicsCounter,
		ctx:                 ctx,
		cancel:              cancel,
	}
}

func (ig *Ingester) Start() {
	for i := 0; i < ig.workers; i++ {
		ig.wait.Go(func() {
			for item := range ig.ingest {
				ig.process(item)
			}
		})
	}
}

func (ig *Ingester) Submit(m models.Metric) error {
	if ig.closed.Load() {
		return ErrIngesterClosed
	}

	select {
	case ig.ingest <- m:
		ig.queueDepthGauge.Inc()
		return nil
	default:
		ig.shedCounter.Inc()
		return ErrQueueFull
	}
}

func (ig *Ingester) Shutdown(ctx context.Context) {
	defer ig.cancel()

	if ig.closed.Swap(true) {
		return
	}

	close(ig.ingest)

	done := make(chan struct{})
	go func() {
		ig.wait.Wait()
		close(done)
	}()

	select {
	case <-done:

	case <-ctx.Done():
		slog.Warn("ingester shutdown deadline expired; canceling in-flight inserts")
		ig.cancel()
		<-done
	}
}

func (ig *Ingester) process(item models.Metric) {
	opCtx, cancel := context.WithTimeout(ig.ctx, 5*time.Second)
	defer cancel()
	defer ig.queueDepthGauge.Dec()
	defer func() {
		if r := recover(); r != nil {
			ig.workerPanicsCounter.Inc()
			slog.Error("worker recovered from panic", "panic", r, "metric_name", item.MetricName)
		}
	}()
	if err := ig.db.InsertMetric(opCtx, item); err != nil {
		slog.Error("Error processing metric insertion", "error", err, "metric_name", item.MetricName)
	} else {
		ig.persistedCounter.Inc()
	}
}
