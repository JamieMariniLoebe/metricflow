# Changelog

All notable changes to MetricFlow are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

> Running on EKS (us-east-2), deployed via GitHub Actions CD pipeline.

### Added

- Kubernetes manifests for EKS deployment (Deployment, Service, StatefulSet, ConfigMap, Secret)
- pgxpool connection pool instrumentation: acquired/idle/max connection counts as gauges, plus cumulative acquire-wait and acquire-duration as counters (all namespaced `metricflow_pgxpool_*`)
- Day 2 Ops: pod kill exercise documented (graceful shutdown verified at 99.2% drain rate under load)
- `metricflow_ingest_persisted_total` counter for successful DB writes at the worker layer; complements `metricflow_ingest_accepted_total` at the handler layer
- POST `/api/metrics` body size cap of 64 KiB via `http.MaxBytesReader`
- JSON decoder rejects requests containing unknown fields (`DisallowUnknownFields`)
- Terraform bootstrap stack: S3 state backend with versioning, encryption, lockfile (`use_lockfile = true`), and public-access block
- Terraform VPC module: 10.0.0.0/16, 3 AZs (us-east-2a/b/c), public + private subnets, single NAT gateway, ELB role tags
- Terraform EKS module: managed node group (t3.medium, 1-3 desired 2), kube-proxy/coredns/vpc-cni addons, IRSA enabled
- Terraform ECR module: immutable tags, 30-image lifecycle policy
- Grafana dashboard panel: latency percentiles (p50/p95/p99) sourced from `http_request_duration_seconds_bucket`
- Grafana dashboard panel: connection pool usage (acquired/idle/max) sourced from `metricflow_pgxpool_*` gauges
- Ingester unit test suite (6 white-box tests: submit accept/shed/closed paths, drain completeness, panic recovery, idempotent shutdown), with a `MetricInserter` interface so the pipeline can be tested behind a fake store
- CD pipeline (`deploy.yml`): OIDC→ECR build, two-stage Trivy scan + SARIF upload, push, EKS deploy with rollout status
- Hardened CI (`ci.yml`): govulncheck, gosec, golangci-lint, SHA-pinned actions
- Terraform RDS, ESO (External Secrets), and GitHub OIDC stacks
- Per-request request_id propagated into structured logs via chi RequestID middleware
- gRPC ingestion endpoint: `MetricsService.IngestMetric` unary RPC on container port 9090 (host 9091 in Compose); shares the ingestion pipeline with the HTTP path. Submit errors map to distinct status codes: `ResourceExhausted` on shed, `Unavailable` on shutdown, `Internal` otherwise.
- gRPC unary handler test suite (4 tests: `TestIngestMetricQueueFull`/`TestIngestMetricHappy`/`TestIngestMetricClosed`/`TestIngestMetricInvalidArgument`), driving the handler as a plain method over a real ingester steered into each error state; proves the error-to-code mapping that the ingester suite couldn't see.
- gRPC ingestion requires mTLS: client cert signed by project CA, server enforces `RequireAndVerifyClientCert`.

### Changed

- Connection pool sizing left at pgxpool defaults after measured no-op finding (peak 5/11 acquired at 200 VUs, zero wait)
- All domain-specific Prometheus metrics namespaced under `metricflow_*` (was a mix of `metrics_*`, `ingest_*`, `pgxpool_*`)
- pgxpool `acquire_wait_seconds` and `acquire_duration_seconds` exposed as counters (`_total` suffix) instead of gauges; underlying values are monotonic, so the gauge form would have produced incorrect `rate()` results
- pgxpool connection-count metric names expanded `_conns` → `_connections`
- `metrics.Metrics` Go struct field names standardized to `<Noun><Type>` form (`IngestQueueDepth` → `QueueDepthGauge`, `IngestRequestsShedTotal` → `ShedCounter`)
- Renamed Prometheus counter `metricflow_ingest_queued_total` → `metricflow_ingest_accepted_total` (breaking, update dashboards/alerts). The counter increments only on successful enqueue, so "accepted" matches semantically. Shed requests are excluded.

### Fixed

- Data race in `/health` handler where the outer-scope `err` variable was written by concurrent request goroutines
- docker-compose Grafana provisioning mount pointed at non-existent `./grafana/provisioning`; now correctly points at `./k8s/grafana`
- Worker goroutines were using `context.Background()`; now inherit context from a per-ingester `context.WithCancel`, and `Shutdown` propagates cancel to in-flight operations
- Double-Shutdown close-of-closed-channel panic: Shutdown now guards on an atomic Swap, so a second call returns before re-closing the channel
- Example gRPC client (`cmd/metricflowclient`) swallowed send failures and exited 0; now reports the error and exits non-zero.

### Known Limitations

- Single-replica deployment with no HorizontalPodAutoscaler or PodDisruptionBudget; multi-replica + HPA pending
- Test coverage limited to the ingest and grpcserver packages: handler and store layers untested (broader integration tests pending)
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
