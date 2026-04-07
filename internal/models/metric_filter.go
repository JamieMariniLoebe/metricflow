package models

import "time"

type MetricFilter struct {
	MetricName string
	MetricType string
	StartTime  time.Time
	EndTime    time.Time
}
