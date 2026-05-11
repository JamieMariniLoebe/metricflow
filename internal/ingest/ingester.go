// Package ingest handles the Async Ingestion Pipeline
package ingest

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/JamieMariniLoebe/metricflow/internal/models"
	"github.com/JamieMariniLoebe/metricflow/internal/store"
	"github.com/prometheus/client_golang/prometheus"
)

type Ingester struct {
	ingest           chan models.Metric
	db               *store.Store
	workers          int
	wait             *sync.WaitGroup
	queueDepthGauge  prometheus.Gauge
	shedCounter      prometheus.Counter
	persistedCounter prometheus.Counter
	ctx              context.Context
	cancel           context.CancelFunc
}

func NewIngester(s *store.Store, w int, queueDepthGauge prometheus.Gauge, shedCounter prometheus.Counter, persistedCounter prometheus.Counter) *Ingester {
	i := make(chan models.Metric, w*5)
	ctx, cancel := context.WithCancel(context.Background())

	return &Ingester{
		ingest:           i,
		db:               s,
		workers:          w,
		wait:             &sync.WaitGroup{},
		queueDepthGauge:  queueDepthGauge,
		shedCounter:      shedCounter,
		persistedCounter: persistedCounter,
		ctx:              ctx,
		cancel:           cancel,
	}
}

func (ig *Ingester) Start() {
	for i := 0; i < ig.workers; i++ {
		ig.wait.Go(func() {
			for item := range ig.ingest {
				opCtx, cancel := context.WithTimeout(ig.ctx, 5*time.Second)
				if err := ig.db.InsertMetric(opCtx, item); err != nil {
					slog.Error("Error processing metric insertion", "error", err, "metric_name", item.MetricName)
				} else {
					ig.persistedCounter.Inc()
				}
				cancel()
				ig.queueDepthGauge.Dec()
			}
		})
	}
}

func (ig *Ingester) Submit(m models.Metric) error {
	select {
	case ig.ingest <- m:
		ig.queueDepthGauge.Inc()
		return nil
	default:
		ig.shedCounter.Inc()
		return errors.New("channel full")
	}
}

func (ig *Ingester) Shutdown(ctx context.Context) {
	defer ig.cancel()

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
