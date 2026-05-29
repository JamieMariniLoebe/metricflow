package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	metricspb "github.com/JamieMariniLoebe/metricflow/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	conn, err := grpc.NewClient("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		slog.Error("client connection failed", "error", err)
		os.Exit(1)
	}

	defer func() {
		if err := conn.Close(); err != nil {
			slog.Error("failed to close connection", "error", err)
		}
	}()

	client := metricspb.NewMetricsServiceClient(conn)

	req := &metricspb.IngestMetricsRequest{
		MetricName: "MetricNameTest",
		MetricType: "MetricTypeTest",
		Labels:     map[string]string{"env": "test"},
		Value:      25,
		MeasuredAt: timestamppb.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.IngestMetric(ctx, req)

	if err != nil {
		slog.Error("ingesting of metric failed", "error", err)
	}

}
