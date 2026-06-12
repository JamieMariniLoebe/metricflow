// Package grpcserver handles metric ingestion over gRPC
package grpcserver

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/JamieMariniLoebe/metricflow/internal/ingest"
	"github.com/JamieMariniLoebe/metricflow/internal/models"
	metricspb "github.com/JamieMariniLoebe/metricflow/proto"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	metricspb.UnimplementedMetricsServiceServer
	ingester        *ingest.Ingester
	acceptedCounter prometheus.Counter
}

func NewServer(ingest prometheus.Counter, ingester *ingest.Ingester) *Server {
	return &Server{
		ingester:        ingester,
		acceptedCounter: ingest,
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
		switch {
		case errors.Is(err, ingest.ErrIngesterClosed):
			slog.Warn("ingester closed", "error", err)
			return nil, status.Error(codes.Unavailable, "Service temporarily unavailable")
		case errors.Is(err, ingest.ErrQueueFull):
			return nil, status.Error(codes.ResourceExhausted, "Too many requests, retry with backoff")
		default:
			slog.Error("unexpected submit error", "error", err)
			return nil, status.Error(codes.Internal, "Internal service error")
		}
	}

	s.acceptedCounter.Inc()

	return &metricspb.IngestMetricsResponse{}, nil
}

func (s *Server) StreamMetrics(stream metricspb.MetricsService_StreamMetricsServer) error {

	var accepted uint64
	var shed uint64
	var rejected uint64

	for {
		req, err := stream.Recv()

		if err != nil {
			switch {
			case errors.Is(err, io.EOF):
				return stream.SendAndClose(&metricspb.StreamMetricsSummary{
					Accepted: accepted,
					Shed:     shed,
					Rejected: rejected,
				})
			default:
				slog.Warn("stream recv failed", "error", err)
				return err
			}
		}

		if req.GetMetricName() == "" || req.GetMetricType() == "" || req.GetMeasuredAt() == nil {
			rejected++
			continue
		}

		met := models.Metric{
			MetricName: req.GetMetricName(),
			MetricType: req.GetMetricType(),
			Labels:     req.GetLabels(),
			Val:        req.GetValue(),
			MeasuredAt: req.GetMeasuredAt().AsTime(),
		}

		err = s.ingester.Submit(met)

		if err != nil {
			switch {
			case errors.Is(err, ingest.ErrIngesterClosed):
				slog.Warn("ingester closed", "error", err)
				return status.Error(codes.Unavailable, "Service temporarily unavailable")
			case errors.Is(err, ingest.ErrQueueFull):
				shed++
				continue
			default:
				slog.Error("unexpected submit error", "error", err)
				return status.Error(codes.Internal, "Internal service error")
			}
		}
		accepted++
		s.acceptedCounter.Inc()
	}
}
