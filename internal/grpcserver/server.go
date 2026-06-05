// Package grpcserver handles metric ingestion over gRPC
package grpcserver

import (
	"context"

	"github.com/JamieMariniLoebe/metricflow/internal/ingest"
	"github.com/JamieMariniLoebe/metricflow/internal/models"
	metricspb "github.com/JamieMariniLoebe/metricflow/proto"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	metricspb.UnimplementedMetricsServiceServer
	ingester      *ingest.Ingester
	queuedCounter prometheus.Counter
}

func NewServer(ingest prometheus.Counter, ingester *ingest.Ingester) *Server {
	return &Server{
		ingester:      ingester,
		queuedCounter: ingest,
	}
}

func (s *Server) IngestMetric(ctx context.Context, req *metricspb.IngestMetricsRequest) (*metricspb.IngestMetricsResponse, error) {

	if req.GetMetricName() == "" || req.GetMetricType() == "" || req.GetMeasuredAt() == nil {
		return nil, status.Error(codes.InvalidArgument, "Missing required field(s)")
	}

	met := models.Metric{
		MetricName: req.GetMetricName(),
		MetricType: req.GetMetricType(),
		Labels:     req.GetLabels(),
		Val:        req.GetValue(),
		MeasuredAt: req.GetMeasuredAt().AsTime(),
	}

	err := s.ingester.Submit(met)

	if err != nil {
		return nil, status.Error(codes.Unavailable, "Service Unavailable")
	}

	s.queuedCounter.Inc()

	return &metricspb.IngestMetricsResponse{}, nil
}
