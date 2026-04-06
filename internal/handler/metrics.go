// Package handler handles HTTP requests
package handler

import (
	"encoding/json"
	"net/http"

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
