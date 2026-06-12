package grpcserver

import (
	"context"
	"testing"
	"time"

	"github.com/JamieMariniLoebe/metricflow/internal/ingest"
	"github.com/JamieMariniLoebe/metricflow/internal/models"
	metricspb "github.com/JamieMariniLoebe/metricflow/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const workers = 5
const bufferCap = workers * 5

type fakeStore struct{}

func (f *fakeStore) InsertMetric(ctx context.Context, m models.Metric) error {
	return nil
}

type testArch struct {
	acceptedCounter prometheus.Counter
	ig              *ingest.Ingester
	srv             *Server
}

func newTestServer() testArch {
	queueDepthGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "metricflow_ingest_queue_depth",
		},
	)
	shedCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "metricflow_ingest_shed_total",
		},
	)
	persistedCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "metricflow_ingest_persisted_total",
		},
	)
	workerPanicsCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "metricflow_worker_panics_total",
		},
	)
	acceptedCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "metricflow_ingest_accepted_total",
		},
	)

	ig := ingest.NewIngester(&fakeStore{}, workers, queueDepthGauge, shedCounter, persistedCounter, workerPanicsCounter)

	return testArch{
		acceptedCounter: acceptedCounter,
		ig:              ig,
		srv:             NewServer(acceptedCounter, ig),
	}
}

func validRequest() *metricspb.IngestMetricsRequest {
	return &metricspb.IngestMetricsRequest{
		MetricName: "grpc test",
		MetricType: "gauge",
		Value:      25.4,
		MeasuredAt: timestamppb.New(time.Now()),
	}
}

func TestIngestMetricQueueFull(t *testing.T) {
	arch := newTestServer()

	for range bufferCap {
		if err := arch.ig.Submit(models.Metric{MetricName: "filer"}); err != nil {
			t.Fatalf("Submit)_ err = %v", err)
		}
	}

	resp, err := arch.srv.IngestMetric(context.Background(), validRequest())

	if resp != nil {
		t.Errorf("resp = %v, want nil", err)
	}
	if got := status.Code(err); got != codes.ResourceExhausted {
		t.Errorf("status.Code(err) = %v, want ResourceExhausted", got)
	}
	if got := testutil.ToFloat64(arch.acceptedCounter); got != 0 {
		t.Errorf("acceptedCounter = %v, want 0", got)
	}
}

func TestIngestMetricHappy(t *testing.T) {
	arch := newTestServer()

	resp, err := arch.srv.IngestMetric(context.Background(), validRequest())

	if err != nil {
		t.Errorf("Error = %v, want nil", err)
	}

	if resp == nil {
		t.Errorf("resp = nil, want non-nil")
	}

	if got := testutil.ToFloat64(arch.acceptedCounter); got != 1 {
		t.Errorf("acceptedCounter = %v, want 1", got)
	}

}

func TestIngestMetricClosed(t *testing.T) {
	arch := newTestServer()

	arch.ig.Shutdown(context.Background())

	resp, err := arch.srv.IngestMetric(context.Background(), validRequest())

	if resp != nil {
		t.Errorf("resp = %v, want nil", resp)
	}

	if got := status.Code(err); got != codes.Unavailable {
		t.Errorf("error = %v, want Unavailable", err)
	}

	if got := testutil.ToFloat64(arch.acceptedCounter); got != 0 {
		t.Errorf("acceptedCounter = %v, want 0", got)
	}

}

func TestIngestMetricInvalidArgument(t *testing.T) {
	arch := newTestServer()

	req := validRequest()
	req.MetricName = ""

	resp, err := arch.srv.IngestMetric(context.Background(), req)

	if resp != nil {
		t.Errorf("resp = %v, want nil", resp)
	}

	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("error = %v, want InvalidArgument", err)
	}

	if got := testutil.ToFloat64(arch.acceptedCounter); got != 0 {
		t.Errorf("acceptedCounter = %v, want 0", got)
	}
}
