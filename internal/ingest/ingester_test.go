package ingest

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/JamieMariniLoebe/metricflow/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type FakeStore struct {
	count    atomic.Int64
	insertFn func(ctx context.Context, m models.Metric) error
}

type testArch struct {
	fake                *FakeStore
	queueDepthGauge     prometheus.Gauge
	shedCounter         prometheus.Counter
	persistedCounter    prometheus.Counter
	workerPanicsCounter prometheus.Counter
	ig                  *Ingester
}

const workers = 5
const bufferCap = workers * 5

func (f *FakeStore) InsertMetric(ctx context.Context, m models.Metric) error {
	f.count.Add(1)
	if f.insertFn != nil {
		return f.insertFn(ctx, m)
	}
	return nil
}

func newTestIngester() testArch {
	fake := &FakeStore{}
	QueueDepthGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "metricflow_ingest_queue_depth",
		},
	)
	ShedCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "metricflow_ingest_shed_total",
		},
	)
	PersistedCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "metricflow_ingest_persisted_total",
		},
	)
	WorkerPanicsCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "metricflow_worker_panics_total",
		},
	)

	ig := NewIngester(fake, workers, QueueDepthGauge, ShedCounter, PersistedCounter, WorkerPanicsCounter)

	return testArch{
		fake:                fake,
		queueDepthGauge:     QueueDepthGauge,
		shedCounter:         ShedCounter,
		persistedCounter:    PersistedCounter,
		workerPanicsCounter: WorkerPanicsCounter,
		ig:                  ig,
	}
}

func TestSubmitHappy(t *testing.T) {
	arch := newTestIngester()

	m := models.Metric{
		MetricName: "Submit Happy Test",
		Val:        25.4,
	}

	err := arch.ig.Submit(m)

	if err != nil {
		t.Errorf("Submit() err = %v, want nil", err)
	}

	got := testutil.ToFloat64(arch.queueDepthGauge)
	if got != 1 {
		t.Errorf("Got = %v, want 1", got)
	}
}

func TestSubmitQueueFull(t *testing.T) {
	arch := newTestIngester()

	m := models.Metric{
		MetricName: "QueueFull Test",
		Val:        25.4,
	}

	for range bufferCap {
		err := arch.ig.Submit(m)
		if err != nil {
			t.Errorf("Submit() err = %v", err)
		}
	}

	err := arch.ig.Submit(m)
	if !errors.Is(err, ErrQueueFull) {
		t.Errorf("Submit() error on ErrQueueFull = %v", err)
	}

	if got := testutil.ToFloat64(arch.shedCounter); got != 1 {
		t.Errorf("Got = %v, want 1", got)
	}

}

func TestIngesterClosed(t *testing.T) {
	arch := newTestIngester()

	arch.ig.Shutdown(context.Background())

	m := models.Metric{
		MetricName: "Ingester closed Test",
		Val:        25.4,
	}

	err := arch.ig.Submit(m)

	if !errors.Is(err, ErrIngesterClosed) {
		t.Errorf("Submit() error = %v, want ErrIngesterClosed", err)
	}
}

func TestWorkersDrain(t *testing.T) {
	arch := newTestIngester()

	m := models.Metric{
		MetricName: "Workers drain Test",
		Val:        25.4,
	}

	for range bufferCap {
		err := arch.ig.Submit(m)
		if err != nil {
			t.Errorf("Submit() err = %v", err)
		}
	}

	arch.ig.Start()

	arch.ig.Shutdown(context.Background())

	if got := arch.fake.count.Load(); got != int64(bufferCap) {
		t.Errorf("Got = %v, want %v", got, bufferCap)
	}

	if got := testutil.ToFloat64(arch.persistedCounter); got != float64(bufferCap) {
		t.Errorf("Got = %v, want %v", got, bufferCap)
	}

	if got := testutil.ToFloat64(arch.queueDepthGauge); got != 0 {
		t.Errorf("Got = %v, want 0", got)
	}
}

func TestDoubleShutdown(t *testing.T) {
	arch := newTestIngester()

	arch.ig.Shutdown(context.Background())
	arch.ig.Shutdown(context.Background())

	// second shutdown must not panic (close-of-closed channel guard)
}

func TestPanicRecover(t *testing.T) {
	arch := newTestIngester()
	var calls atomic.Int64

	met1 := models.Metric{
		MetricName: "Test Panic",
		Val:        25.4,
	}

	met2 := models.Metric{
		MetricName: "Test Recover",
		Val:        25.4,
	}

	arch.fake.insertFn = func(ctx context.Context, m models.Metric) error {
		if calls.Add(1) == 1 {
			panic("Panicking!")
		}
		return nil
	}

	err := arch.ig.Submit(met1)
	if err != nil {
		t.Errorf("Submit() err = %v", err)
	}

	err = arch.ig.Submit(met2)
	if err != nil {
		t.Errorf("Submit() err = %v", err)
	}

	arch.ig.Start()

	arch.ig.Shutdown(context.Background())

	if got := testutil.ToFloat64(arch.workerPanicsCounter); got != 1 {
		t.Errorf("Got = %v, want 1", got)
	}

	if got := testutil.ToFloat64(arch.persistedCounter); got != 1 {
		t.Errorf("Got = %v, want 1", got)
	}

}
