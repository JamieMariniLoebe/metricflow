// Package ingest handles the Async Ingestion Pipeline
package ingest

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/JamieMariniLoebe/metricflow/internal/models"
	"github.com/JamieMariniLoebe/metricflow/internal/store"
)

type Ingester struct {
	ingest  chan models.Metric
	db      *store.Store
	workers int
	wait    *sync.WaitGroup
}

func NewIngester(s *store.Store, w int) *Ingester {
	i := make(chan models.Metric, w*5)

	return &Ingester{
		ingest:  i,
		db:      s,
		workers: w,
		wait:    &sync.WaitGroup{},
	}
}

func (ig *Ingester) Start(c context.Context) {
	for i := 0; i < ig.workers; i++ {
		ig.wait.Go(func() {
			for item := range ig.ingest {
				err := ig.db.InsertMetric(c, item)
				if err != nil {
					slog.Error("Error processing metric insertion", "error", err, "metric_name", item.MetricName)
				}
			}
		})
	}
}

func (ig *Ingester) Submit(m models.Metric) error {
	select {
	case ig.ingest <- m:
		return nil
	default:
		return errors.New("channel full")
	}
}

func (ig *Ingester) Shutdown() {
	close(ig.ingest)
	ig.wait.Wait()
}
