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
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}

}

func run(ctx context.Context) error {
	sourceURL := os.Getenv("SOURCE_URL")
	if sourceURL == "" {
		return fmt.Errorf("empty source_url")
	}
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	if host == "" {
		return fmt.Errorf("empty host var")
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		return fmt.Errorf("empty port var")
	}
	name := os.Getenv("DB_NAME")
	if name == "" {
		return fmt.Errorf("empty name var")
	}
	sslMode := os.Getenv("DB_SSLMODE")
	if sslMode == "" {
		sslMode = "require"
	}

	url := fmt.Sprintf("postgres://%s:%s/%s?sslmode=%s", host, port, name, sslMode)

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return fmt.Errorf("failed to parse db config: %w", err)
	}

	cfg.ConnConfig.User = user
	cfg.ConnConfig.Password = password

	sqlDB := stdlib.OpenDB(*cfg.ConnConfig)

	if err := database.RunMigrations(sqlDB, sourceURL); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		slog.Warn("failed to close migration db", "error", err)
	}

	db, err := store.NewPool(cfg)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}

	defer db.Close()

	reg := prometheus.NewRegistry()

	m := metrics.NewMetrics(reg, db)

	s := store.NewStore(db)

	i := ingest.NewIngester(s, 5, m.QueueDepthGauge, m.ShedCounter, m.PersistedCounter, m.WorkerPanicsCounter)

	h := handler.NewHandler(s, m.AcceptedCounter, i)

	grpcServer := grpc.NewServer()

	reflection.Register(grpcServer)

	i.Start()

	r := chi.NewRouter()

	srv := &http.Server{
		ReadHeaderTimeout: 10 * time.Second, // mitigates Slowloris (G112)
		Addr:              ":8080",
		Handler:           r,
	}

	grpcSvc := grpcserver.NewServer(m.AcceptedCounter, i)

	metricspb.RegisterMetricsServiceServer(grpcServer, grpcSvc)

	r.Use(middleware.RequestID)
	r.Use(m.Middleware)

	r.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("failed to encode health response", "error", err)
		}
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		if err := db.Ping(ctx); err != nil {
			slog.Error("readiness check failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r.Post("/api/metrics", h.CreateMetric)

	r.Get("/api/metrics", h.GetMetrics)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		lis, err := net.Listen("tcp", ":9090") // #nosec G102 - intentional: gRPC server listens on all interfaces; exposure controlled by Kubernetes NetworkPolicy
		if err != nil {
			return fmt.Errorf("gRPC listen failed: %w", err)
		}

		if err := grpcServer.Serve(lis); err != nil {
			return fmt.Errorf("gRPC server failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP listen failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown failed", "error", err)
		}

		grpcServer.GracefulStop()

		i.Shutdown(shutdownCtx)

		slog.Info("shutdown", "reason", context.Cause(ctx))
		return nil
	})

	slog.Info("MetricFlow starting", "http_port", 8080, "grpc_port", 9090)

	return g.Wait()

}
