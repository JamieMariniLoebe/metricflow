// Package handler manages metric ingestion and queries over HTTP/JSON
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/JamieMariniLoebe/metricflow/internal/ingest"
	"github.com/JamieMariniLoebe/metricflow/internal/models"
	"github.com/JamieMariniLoebe/metricflow/internal/store"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

// Handler holds dependencies for HTTP request handlers
type Handler struct {
	store         *store.Store
	queuedCounter prometheus.Counter
	ingester      *ingest.Ingester
}

// NewHandler creates a Handler with the given store for database access
func NewHandler(store *store.Store, ingest prometheus.Counter, ingester *ingest.Ingester) *Handler {
	return &Handler{
		store:         store,
		queuedCounter: ingest,
		ingester:      ingester,
	}
}

// CreateMetric handles POST requests to ingest a new metric data point
func (h *Handler) CreateMetric(w http.ResponseWriter, r *http.Request) {
	logger := slog.With("request_id", middleware.GetReqID(r.Context()))

	decode := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	decode.DisallowUnknownFields()
	var met models.Metric

	err := decode.Decode(&met)

	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if met.MetricName == "" || met.MetricType == "" || met.MeasuredAt.IsZero() {
		http.Error(w, "Missing required field(s)", http.StatusUnprocessableEntity)
		return
	}

	err = h.ingester.Submit(met)

	if err != nil {
		switch {
		case errors.Is(err, ingest.ErrIngesterClosed):
			logger.Warn("ingester closed", "error", err)
			http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		case errors.Is(err, ingest.ErrQueueFull):
			http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		default:
			logger.Error("Unexpected submission error", "error", err)
			http.Error(w, "Internal service error", http.StatusInternalServerError)
		}
		return
	}

	h.queuedCounter.Inc()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	err = json.NewEncoder(w).Encode(met)

	if err != nil {
		logger.Error("Failed to encode metric response", "error", err)
	}
}

// GetMetrics handles GET requests to query metrics with optional filters
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	logger := slog.With("request_id", middleware.GetReqID(r.Context()))

	params := r.URL.Query()

	var filter models.MetricFilter
	var err error
	var metrics []models.Metric

	if params.Get("metric_name") != "" {
		filter.MetricName = params.Get("metric_name")
	}

	if params.Get("metric_type") != "" {
		filter.MetricType = params.Get("metric_type")
	}

	if params.Get("start_time") != "" {
		filter.StartTime, err = time.Parse(time.RFC3339, params.Get("start_time"))

		if err != nil {
			http.Error(w, "Invalid start_time format", http.StatusBadRequest)
			return
		}
	}

	if params.Get("end_time") != "" {
		filter.EndTime, err = time.Parse(time.RFC3339, params.Get("end_time"))

		if err != nil {
			http.Error(w, "Invalid end_time format", http.StatusBadRequest)
			return
		}
	}

	opCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	metrics, err = h.store.GetMetrics(opCtx, filter)

	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			logger.Debug("context cancelled", "error", err)
			http.Error(w, "Internal service error", 499)
		case errors.Is(err, context.DeadlineExceeded):
			logger.Warn("deadline exceeded", "error", err)
			http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		default:
			logger.Error("query metrics failed", "error", err)
			http.Error(w, "Internal service error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(metrics)
	if err != nil {
		logger.Error("Failed to encode metrics response", "error", err)
	}
}
