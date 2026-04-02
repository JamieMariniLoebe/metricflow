// package handler handles HTTP requests
package handler

import (
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
