package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/JamieMariniLoebe/metricflow/internal/database"
	"github.com/JamieMariniLoebe/metricflow/internal/handler"
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

	reg := prometheus.NewRegistry()

	m := metrics.NewMetrics(reg)

	s := store.NewStore(db)

	h := handler.NewHandler(s, m.IngestedCounter)

	r := chi.NewRouter()

	r.Use(m.Middleware)

	r.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	r.Post("/api/metrics", h.CreateMetric)

	r.Get("/api/metrics", h.GetMetrics)

	slog.Info("MetricFlow starting", "port", 8080)

	err = http.ListenAndServe(":8080", r)

	slog.Error("Listen and serve failed", "error", err)
	os.Exit(1)

}
