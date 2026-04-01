package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/JamieMariniLoebe/metricflow/internal/models"
)

type Store struct {
	db *sql.DB
}

func NewStore(pool *sql.DB) *Store {
	return &Store{
		db: pool,
	}
}

func (s *Store) InsertMetric(ctx context.Context, metric models.Metric) error {
	postgresLabels, err := json.Marshal(metric.Labels)

	query := "INSERT INTO metrics (metric_name, metric_type, labels, val, measured_at) VALUES ($1, $2, $3, $4, $5)"

	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, query, metric.MetricName, metric.MetricType, postgresLabels, metric.Val, metric.MeasuredAt)

	return err
}
