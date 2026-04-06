package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/JamieMariniLoebe/metricflow/internal/database"
	"github.com/JamieMariniLoebe/metricflow/internal/handler"
	"github.com/JamieMariniLoebe/metricflow/internal/store"
	"github.com/go-chi/chi/v5"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	sourceURL := os.Getenv("SOURCE_URL")

	if dbURL == "" {
		log.Fatal("Empty database_url")
	}

	if sourceURL == "" {
		log.Fatal("Empty source_url")
	}

	pgxURL := strings.Replace(dbURL, "postgres://", "pgx5://", 1)

	if err := database.RunMigrations(pgxURL, sourceURL); err != nil {
		log.Fatal("Error")
	}

	db, err := store.NewPool(dbURL)

	if err != nil {
		log.Fatal("Error")
	}

	defer db.Close()

	s := store.NewStore(db)
	h := handler.NewHandler(s)

	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	r.Post("/metrics", h.CreateMetric)

	log.Println("MetricFlow starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))

}
