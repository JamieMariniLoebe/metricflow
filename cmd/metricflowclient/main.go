package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"time"

	metricspb "github.com/JamieMariniLoebe/metricflow/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal client error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	clientCert, err := tls.LoadX509KeyPair("certs/client.crt", "certs/client.key")
	if err != nil {
		return err
	}

	caBytes, err := os.ReadFile("certs/ca.crt")
	if err != nil {
		return err
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caBytes) {
		return fmt.Errorf("failed to add CA cert to pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
	}

	conn, err := grpc.NewClient("localhost:9090", grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))

	if err != nil {
		return fmt.Errorf("client connection failed: %w", err)
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
		return fmt.Errorf("ingest failed: %w", err)
	}

	return nil

}
