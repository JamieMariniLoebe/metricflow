package store

import (
	"context"
	"encoding/json"

	"github.com/JamieMariniLoebe/metricflow/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
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

	_, err = s.db.Exec(ctx, query, metric.MetricName, metric.MetricType, postgresLabels, metric.Val, metric.MeasuredAt)

	return err
}
