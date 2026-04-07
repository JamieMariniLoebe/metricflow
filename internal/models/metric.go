// Package models defines data structures for the metrics API
package models

import "time"

// Metric represents a single metric data point with a name, type, labels, value and timestamp
type Metric struct {
	ID         int               `json:"-"`
	MetricName string            `json:"metric_name"`
	MetricType string            `json:"metric_type"`
	Labels     map[string]string `json:"labels"`
	Val        float64           `json:"val"`
	CreatedAt  time.Time         `json:"-"`
	MeasuredAt time.Time         `json:"measured_at"`
}
