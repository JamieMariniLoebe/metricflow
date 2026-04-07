// Package handler handles HTTP requests
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/JamieMariniLoebe/metricflow/internal/models"
	"github.com/JamieMariniLoebe/metricflow/internal/store"
)

type Handler struct {
	store *store.Store
}

func NewHandler(store *store.Store) *Handler {
	return &Handler{
		store: store,
	}
}

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

	err = h.store.InsertMetric(r.Context(), met)

	if err != nil {
		http.Error(w, "Internal service error", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(met)
}

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

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metrics)
}
