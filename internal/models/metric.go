// Package models defines metric structure
package models

import "time"

type Metric struct {
	ID         int               `json:"-"`
	MetricName string            `json:"metric_name"`
	MetricType string            `json:"metric_type"`
	Labels     map[string]string `json:"labels"`
	Val        float64           `json:"val"`
	CreatedAt  time.Time         `json:"-"`
	MeasuredAt time.Time         `json:"measured_at"`
}
