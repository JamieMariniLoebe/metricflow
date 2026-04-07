package models

import "time"

// MetricFilter holds optional filters for querying the database
type MetricFilter struct {
	MetricName string
	MetricType string
	StartTime  time.Time
	EndTime    time.Time
}
