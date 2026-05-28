package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JamieMariniLoebe/metricflow/internal/database"
	"github.com/JamieMariniLoebe/metricflow/internal/grpcserver"
	"github.com/JamieMariniLoebe/metricflow/internal/handler"
	"github.com/JamieMariniLoebe/metricflow/internal/ingest"
	"github.com/JamieMariniLoebe/metricflow/internal/metrics"
	"github.com/JamieMariniLoebe/metricflow/internal/store"
	metricspb "github.com/JamieMariniLoebe/metricflow/proto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

func main() {

	sourceURL := os.Getenv("SOURCE_URL")
	if sourceURL == "" {
		slog.Error("Empty source_url")
		os.Exit(1)
	}
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	if host == "" {
		slog.Error("Empty host var")
		os.Exit(1)
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		slog.Error("Empty port var")
		os.Exit(1)
	}
	name := os.Getenv("DB_NAME")
	if name == "" {
		slog.Error("Empty name var")
		os.Exit(1)
	}

	url := fmt.Sprintf("postgres://%s:%s/%s?sslmode=require", host, port, name)

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		slog.Error("Failed to parse db config", "error", err)
		os.Exit(1)
	}

	cfg.ConnConfig.User = user
	cfg.ConnConfig.Password = password

	sqlDB := stdlib.OpenDB(*cfg.ConnConfig)

	if err := database.RunMigrations(sqlDB, sourceURL); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	if err := sqlDB.Close(); err != nil {
		slog.Warn("failed to close migration db", "error", err)
	}

	db, err := store.NewPool(cfg)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}

	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	reg := prometheus.NewRegistry()

	m := metrics.NewMetrics(reg, db)

	s := store.NewStore(db)

	i := ingest.NewIngester(s, 5, m.QueueDepthGauge, m.ShedCounter, m.PersistedCounter, m.WorkerPanicsCounter)

	h := handler.NewHandler(s, m.QueuedCounter, i)

	grpcServer := grpc.NewServer()

	i.Start()

	r := chi.NewRouter()

	grpcSvc := grpcserver.NewServer(s, m.QueuedCounter, i)

	metricspb.RegisterMetricsServiceServer(grpcServer, grpcSvc)

	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		slog.Error("gRPC listen failed", "error", err)
		os.Exit(1)
	}

	r.Use(m.Middleware)

	r.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode health response", "error", err)
		}
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		if err := db.Ping(ctx); err != nil {
			slog.Error("Readiness check failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r.Post("/api/metrics", h.CreateMetric)

	r.Get("/api/metrics", h.GetMetrics)

	slog.Info("MetricFlow starting", "http_port", 8080, "grpc_port", 9090)

	srv := &http.Server{
		ReadHeaderTimeout: 10 * time.Second, // mitigates Slowloris (G112)
		Addr:              ":8080",
		Handler:           r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			stop()
		}
	}()

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown failed", "error", err)
	}

	i.Shutdown(shutdownCtx)

	slog.Info("Shutdown....", "reason", context.Cause(ctx))

}
