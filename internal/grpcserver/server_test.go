package grpcserver

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/JamieMariniLoebe/metricflow/internal/ingest"
	"github.com/JamieMariniLoebe/metricflow/internal/models"
	metricspb "github.com/JamieMariniLoebe/metricflow/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const workers = 5
const bufferCap = workers * 5

type fakeStore struct{}

type fakeStream struct {
	grpc.ServerStream
	summary *metricspb.StreamMetricsSummary
	reqs    []*metricspb.IngestMetricsRequest
	cursor  int
}

type testArch struct {
	acceptedCounter prometheus.Counter
	ig              *ingest.Ingester
	srv             *Server
}

func (f *fakeStore) InsertMetric(ctx context.Context, m models.Metric) error {
	return nil
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

func (f *fakeStream) Recv() (*metricspb.IngestMetricsRequest, error) {
	if f.cursor >= len(f.reqs) {
		return nil, io.EOF
	}
	req := f.reqs[f.cursor]
	f.cursor++
	return req, nil
}

func (f *fakeStream) SendAndClose(summary *metricspb.StreamMetricsSummary) error {
	f.summary = summary
	return nil
}

func TestMetricStreamHappy(t *testing.T) {
	arch := newTestServer()

	req1 := validRequest()
	req2 := validRequest()
	req3 := validRequest()

	fake := fakeStream{
		reqs: []*metricspb.IngestMetricsRequest{req1, req2, req3},
	}
	err := arch.srv.StreamMetrics(&fake)

	if err != nil {
		t.Errorf("error: %v", err)
	}

	if got := fake.summary.Accepted; got != 3 {
		t.Errorf("Accepted = %v, want 3", got)
	}

	if got := fake.summary.Rejected; got != 0 {
		t.Errorf("Rejected = %v, want 0", got)
	}

	if got := fake.summary.Shed; got != 0 {
		t.Errorf("Shed = %v, want 0", got)
	}

	if got := testutil.ToFloat64(arch.acceptedCounter); got != 3 {
		t.Errorf("acceptedCounter = %v, want 3", got)
	}
}

func TestMetricStreamRejected(t *testing.T) {
	arch := newTestServer()

	req1 := validRequest()
	req2 := validRequest()
	req3 := validRequest()

	req2.MetricName = ""

	fake := fakeStream{
		reqs: []*metricspb.IngestMetricsRequest{req1, req2, req3},
	}

	err := arch.srv.StreamMetrics(&fake)

	if err != nil {
		t.Errorf("error: %v", err)
	}

	if got := fake.summary.Accepted; got != 2 {
		t.Errorf("Accepted = %v, want 2", got)
	}

	if got := fake.summary.Rejected; got != 1 {
		t.Errorf("Rejected = %v, want 1", got)
	}

	if got := fake.summary.Shed; got != 0 {
		t.Errorf("Shed = %v, want 0", got)
	}

	if got := testutil.ToFloat64(arch.acceptedCounter); got != 2 {
		t.Errorf("acceptedCounter = %v, want 2", got)
	}

}

func TestMetricStreamShed(t *testing.T) {
	arch := newTestServer()

	for range bufferCap {
		if err := arch.ig.Submit(models.Metric{MetricName: "filer"}); err != nil {
			t.Fatalf("Submit)_ err = %v", err)
		}
	}

	req1 := validRequest()
	req2 := validRequest()
	req3 := validRequest()

	fake := fakeStream{
		reqs: []*metricspb.IngestMetricsRequest{req1, req2, req3},
	}

	err := arch.srv.StreamMetrics(&fake)

	if err != nil {
		t.Errorf("error: %v", err)
	}

	if got := fake.summary.Shed; got != 3 {
		t.Errorf("Shed = %v, want 3", got)
	}
}

func TestMetricStreamClosed(t *testing.T) {
	arch := newTestServer()

	arch.ig.Shutdown(context.Background())

	req1 := validRequest()
	req2 := validRequest()
	req3 := validRequest()

	fake := fakeStream{
		reqs: []*metricspb.IngestMetricsRequest{req1, req2, req3},
	}
	err := arch.srv.StreamMetrics(&fake)

	if got := status.Code(err); got != codes.Unavailable {
		t.Errorf("error = %v, want Unavailable", err)
	}
}
