package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/JamieMariniLoebe/metricflow/internal/database"
	"github.com/JamieMariniLoebe/metricflow/internal/handler"
	"github.com/JamieMariniLoebe/metricflow/internal/ingest"
	"github.com/JamieMariniLoebe/metricflow/internal/metrics"
	"github.com/JamieMariniLoebe/metricflow/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	sourceURL := os.Getenv("SOURCE_URL")

	if dbURL == "" {
		slog.Error("Empty database_url")
		os.Exit(1)
	}

	if sourceURL == "" {
		slog.Error("Empty source_url")
		os.Exit(1)
	}

	pgxURL := strings.Replace(dbURL, "postgres://", "pgx5://", 1)

	if err := database.RunMigrations(pgxURL, sourceURL); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	db, err := store.NewPool(dbURL)

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

	i := ingest.NewIngester(s, 5, m.QueueDepthGauge, m.ShedCounter, m.PersistedCounter)

	h := handler.NewHandler(s, m.QueuedCounter, i)

	i.Start()

	r := chi.NewRouter()

	r.Use(m.Middleware)

	r.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode health response", "error", err)
		}
	})

	r.Post("/api/metrics", h.CreateMetric)

	r.Get("/api/metrics", h.GetMetrics)

	slog.Info("MetricFlow starting", "port", 8080)

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

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown failed", "error", err)
	}

	i.Shutdown(shutdownCtx)

	slog.Info("Shutdown....", "reason", context.Cause(ctx))

}
