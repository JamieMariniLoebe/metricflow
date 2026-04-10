package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/JamieMariniLoebe/metricflow/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store holds a PostgreSQL connection pool and provides data access methods
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a Store with the given connection pool
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		db: pool,
	}
}

// InsertMetric ingests a new metric data point into the database
func (s *Store) InsertMetric(ctx context.Context, metric models.Metric) error {
	postgresLabels, err := json.Marshal(metric.Labels)

	if err != nil {
		return err
	}

	query := "INSERT INTO metrics (metric_name, metric_type, labels, val, measured_at) VALUES ($1, $2, $3, $4, $5)"

	_, err = s.db.Exec(ctx, query, metric.MetricName, metric.MetricType, postgresLabels, metric.Val, metric.MeasuredAt)

	return err
}

// GetMetrics queries metric data points from the database using optional filters
func (s *Store) GetMetrics(ctx context.Context, filter models.MetricFilter) ([]models.Metric, error) {
	var args []any
	var conditions []string

	q := "SELECT id, metric_name, metric_type, labels, val, created_at, measured_at FROM metrics"

	if filter.MetricName != "" {
		args = append(args, filter.MetricName)
		conditions = append(conditions, fmt.Sprintf("metric_name = $%d", len(args)))
	}

	if filter.MetricType != "" {
		args = append(args, filter.MetricType)
		conditions = append(conditions, fmt.Sprintf("metric_type = $%d", len(args)))
	}

	if !filter.StartTime.IsZero() {
		args = append(args, filter.StartTime)
		conditions = append(conditions, fmt.Sprintf("measured_at >= $%d", len(args)))
	}

	if !filter.EndTime.IsZero() {
		args = append(args, filter.EndTime)
		conditions = append(conditions, fmt.Sprintf("measured_at <= $%d", len(args)))
	}

	if len(conditions) != 0 {
		q += " WHERE "
		q += strings.Join(conditions, " AND ")
	}

	r, err := s.db.Query(ctx, q, args...)

	if err != nil {
		return nil, err
	}

	defer r.Close()

	var metrics []models.Metric

	for r.Next() {
		var m models.Metric
		err := r.Scan(&m.ID, &m.MetricName, &m.MetricType, &m.Labels, &m.Val, &m.CreatedAt, &m.MeasuredAt)

		if err != nil {
			return nil, err
		}

		metrics = append(metrics, m)
	}

	err = r.Err()

	if err != nil {
		return nil, err
	}

	return metrics, nil

}
