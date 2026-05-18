# Changelog

All notable changes to MetricFlow are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

> Phase 2B in progress: AWS infrastructure provisioned via Terraform (not yet the active runtime); deployment cutover from Minikube to EKS pending.

### Added

- Kubernetes manifests for Minikube deployment (Deployment, Service, StatefulSet, ConfigMap, Secret)
- pgxpool connection pool instrumentation: acquired/idle/max connection counts as gauges, plus cumulative acquire-wait and acquire-duration as counters (all namespaced `metricflow_pgxpool_*`)
- Day 2 Ops: pod kill exercise documented (graceful shutdown verified at 99.2% drain rate under load)
- `metricflow_ingest_persisted_total` counter for successful DB writes at the worker layer; complements `metricflow_ingest_queued_total` at the handler layer
- POST `/api/metrics` body size cap of 64 KiB via `http.MaxBytesReader`
- JSON decoder rejects requests containing unknown fields (`DisallowUnknownFields`)
- Terraform bootstrap stack: S3 state backend with versioning, encryption, lockfile (`use_lockfile = true`), and public-access block
- Terraform VPC module: 10.0.0.0/16, 3 AZs (us-east-2a/b/c), public + private subnets, single NAT gateway, ELB role tags
- Terraform EKS module: managed node group (t3.medium, 1-3 desired 2), kube-proxy/coredns/vpc-cni addons, IRSA enabled
- Terraform ECR module: immutable tags, 30-image lifecycle policy
- Grafana dashboard panel: latency percentiles (p50/p95/p99) sourced from `http_request_duration_seconds_bucket`
- Grafana dashboard panel: connection pool usage (acquired/idle/max) sourced from `metricflow_pgxpool_*` gauges

### Changed

- Connection pool sizing left at pgxpool defaults after measured no-op finding (peak 5/11 acquired at 200 VUs, zero wait)
- All domain-specific Prometheus metrics namespaced under `metricflow_*` (was a mix of `metrics_*`, `ingest_*`, `pgxpool_*`)
- pgxpool `acquire_wait_seconds` and `acquire_duration_seconds` exposed as counters (`_total` suffix) instead of gauges; underlying values are monotonic, so the gauge form would have produced incorrect `rate()` results
- pgxpool connection-count metric names expanded `_conns` → `_connections`
- `metrics.Metrics` Go struct field names standardized to `<Noun><Type>` form (`IngestQueueDepth` → `QueueDepthGauge`, `IngestRequestsShedTotal` → `ShedCounter`)

### Fixed

- Data race in `/health` handler where the outer-scope `err` variable was written by concurrent request goroutines
- docker-compose Grafana provisioning mount pointed at non-existent `./grafana/provisioning`; now correctly points at `./k8s/grafana`
- Worker goroutines were using `context.Background()`; now inherit context from a per-ingester `context.WithCancel`, and `Shutdown` propagates cancel to in-flight operations

### Known Limitations

- Request IDs not propagated through `slog`
- Single-replica deployment with no HorizontalPodAutoscaler or PodDisruptionBudget; multi-replica + HPA pending Phase 2B / EKS migration
- No automated test suite; unit + integration tests scheduled for Phase 3
- Database schema lacks NOT NULL constraints and a `(metric_name, measured_at)` index — pending follow-up

## [0.2.0] — 2026-04-22 — Phase 2A Close

### Added

- Buffered channel + worker pool ingestion pipeline (25-slot buffer, 5 workers)
- 503 load shedding when channel full, with `ingest_requests_shed_total` counter
- Graceful shutdown on SIGTERM/SIGINT
- k6 load testing with 10 → 50 → 200 VU ramp profile
- Grafana dashboard (5 panels) auto-provisioned via Docker Compose
- Multi-stage Dockerfile producing 8.8MB scratch image

### Changed

- Migrated database driver from `lib/pq` + `database/sql` to `pgx/v5` + `pgxpool`

## [0.1.0] — 2026-04-10 — Phase 1 Foundation

### Added

- Initial Go HTTP service scaffold using Chi router
- POST `/api/metrics` and GET `/api/metrics` endpoints
- PostgreSQL persistence with JSONB labels column
- Prometheus instrumentation: request counter, latency histogram
- `slog` structured logging
- `golang-migrate` schema migrations
