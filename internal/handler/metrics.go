// Package handler processes HTTP requests
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/JamieMariniLoebe/metricflow/internal/ingest"
	"github.com/JamieMariniLoebe/metricflow/internal/models"
	"github.com/JamieMariniLoebe/metricflow/internal/store"
	"github.com/prometheus/client_golang/prometheus"
)

// Handler holds dependencies for HTTP request handlers
type Handler struct {
	store    *store.Store
	ingest   prometheus.Counter
	ingester *ingest.Ingester
}

// NewHandler creates a Handler with the given store for database access
func NewHandler(store *store.Store, ingest prometheus.Counter, ingester *ingest.Ingester) *Handler {
	return &Handler{
		store:    store,
		ingest:   ingest,
		ingester: ingester,
	}
}

// CreateMetric handles POST requests to ingest a new metric data point
func (h *Handler) CreateMetric(w http.ResponseWriter, r *http.Request) {
	decode := json.NewDecoder(r.Body)
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
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	h.ingest.Inc()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	err = json.NewEncoder(w).Encode(met)

	if err != nil {
		slog.Error("Failed to encode metric response", "error", err)
	}
}

// GetMetrics handles GET requests to query metrics with optional filters
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()

	var filter models.MetricFilter
	var err error
	metrics := []models.Metric{}

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

	metrics, err = h.store.GetMetrics(r.Context(), filter)

	if err != nil {
		http.Error(w, "Internal service error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(metrics)
	if err != nil {
		slog.Error("Failed to encode metrics response", "error", err)
	}
}
