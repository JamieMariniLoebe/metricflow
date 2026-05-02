# Changelog

All notable changes to MetricFlow are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Kubernetes manifests for Minikube deployment (Deployment, Service, StatefulSet, ConfigMap, Secret)
- pgxpool connection pool instrumentation: acquired, idle, max, wait, and acquire-duration gauges
- Day 2 Ops: pod kill exercise documented (graceful shutdown verified at 99.2% drain rate under load)

### Changed

- Connection pool sizing left at pgxpool defaults after measured no-op finding (peak 5/11 acquired at 200 VUs, zero wait)

### Known Limitations

- Workers use `context.Background()` instead of inheriting parent context; 0.8% drain loss observed during pod kill
- Request IDs not propagated through `slog`
- Single-instance deployment; multi-instance / EKS pending

## [0.2.0] — 2026-04-XX — Phase 2A Close

### Added

- Buffered channel + worker pool ingestion pipeline (25-slot buffer, 5 workers)
- 503 load shedding when channel full, with `ingest_requests_shed_total` counter
- Graceful shutdown on SIGTERM/SIGINT
- k6 load testing with 10 → 50 → 200 VU ramp profile
- Grafana dashboard (5 panels) auto-provisioned via Docker Compose
- Multi-stage Dockerfile producing 8.8MB scratch image

### Changed

- Migrated database driver from `lib/pq` + `database/sql` to `pgx/v5` + `pgxpool`

## [0.1.0] — 2026-04-XX — Phase 1 Foundation

### Added

- Initial Go HTTP service scaffold using Chi router
- POST `/api/metrics` and GET `/api/metrics` endpoints
- PostgreSQL persistence with JSONB labels column
- Prometheus instrumentation: request counter, latency histogram
- `slog` structured logging
- `golang-migrate` schema migrations
