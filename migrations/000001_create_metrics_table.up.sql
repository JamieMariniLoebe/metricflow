CREATE TABLE metrics (
    id BIGSERIAL PRIMARY KEY,
    metric_name TEXT,
    metric_type VARCHAR(50),
    labels JSONB,
    val NUMERIC,
    created_at TIMESTAMPTZ, DEFAULT CURRENT_
    measured_at TIMESTAMPTZ
)