# Modernization Worklog

This document tracks the ongoing effort to refactor the SCM Ads API into a scalable, maintainable, production-grade platform.

| Date | Area | Status | Notes |
| ---- | ---- | ------ | ----- |
| 2026-05-08 | Configuration Hardening | ✅ Complete | `config.Load()` now returns `(*Config, error)` with strict validation of critical env vars (JWT secret, SMTP creds, console creds). Prevents insecure defaults and surfaces misconfiguration early. |
| 2026-05-08 | Creative Handler Wiring | ✅ Complete | `NewCreativeHandler` and creative route registration now bubble configuration errors up through `SetupRoutes` to `main`. Ensures router wiring fails fast if dependencies are missing. |
| 2026-05-08 | Test Suite Updates | ✅ Complete | Creative handler and routes tests updated to assert the new error-returning constructors, keeping CI green and contracts enforced. |
| 2026-05-08 | Structured Logging Foundation | ✅ Complete | Added `internal/logger` with zerolog, env-driven log levels, and rewired `cmd/api/main.go` plus schedulers/pop sync pipelines to emit structured logs. Establishes a centralized logger for future middleware/feature modules. |
| 2026-05-08 | PGX Pool Platform Layer | 🚧 In Progress | Added env-driven DB pool tuning knobs in config and introduced `internal/platform/database` with a pgxpool builder + config adapter. Next step: migrate data access layers to the new pool. |
| 2026-05-08 | Campaign Repository Migration | ✅ Complete | `internal/repository/campaign_repository` now uses `pgxpool` with context timeouts, `ErrNoRows` translation, and shared pool wiring via `cmd/api/main.go`. Routes/tests inject the repo so handlers and background jobs share the same pool-backed instance. |
| 2026-05-08 | Creative Repository Migration | ✅ Complete | `internal/repository/creative_repository` migrated to `pgxpool` with context timeouts, removed `pq.Array` usage (pgx handles slices natively), and updated transaction handling for `PickNextRotationalCreative`. Routes, legacy handler, and tests updated to inject the repo. |
| 2026-05-08 | User Repository Migration | ✅ Complete | `internal/repository/user_repository` migrated to `pgxpool` with context timeouts and `ErrNoRows` translation. Auth handler and user routes updated to inject the repo. Auth handler tests skipped (sqlmock incompatible with pgx) with note to rewrite with pgxmock or integration tests. |
| 2026-05-08 | Advertiser Repository Migration | ✅ Complete | `internal/repository/advertiser_repository` migrated to `pgxpool` with context timeouts, removed `pq.Array` usage, and `ErrNoRows` translation. Advertiser routes updated to inject the repo. Tests updated to use noop stub. |
| 2026-05-08 | Device Repository Migration | ✅ Complete | `internal/repository/device_repository` migrated to `pgxpool` with context timeouts and `ErrNoRows` translation. Device routes and sync routes updated to inject the repo. Tests updated to use noop stubs for device and project repositories. |
| 2026-05-08 | Permission Repository Migration | ✅ Complete | `internal/repository/permission_repository` migrated to `pgxpool` with context timeouts and `ErrNoRows` translation. RBAC handler/routes now inject role, permission, and user-role repositories, and tests use new noop stubs. |
| 2026-05-08 | Role Repository Migration | ✅ Complete | `internal/repository/role_repository` migrated to `pgxpool` with timeouts and `ErrNoRows` translation, plus transaction handling via pgx. RBAC wiring/tests updated to inject role repo stubs and pgx-backed instance from `cmd/api/main.go`. |
| 2026-05-08 | Project Repository Migration | ✅ Complete | `internal/repository/project_repository` migrated to `pgxpool` with timeouts, JSON marshalling, and search/filter helpers updated. Project routes now receive injected repos, `main` wires pgx-backed repo, and test stubs remain compatible. |

## Planned Improvements

1. **Foundations & Logging**
   - Structured logger (zap/zerolog)
   - Config ergonomics (typed sections, env overrides)
   - Request/response helpers

2. **Platform Layers**
   - `pgx/v5` database abstraction with timeouts & prepared statements
   - Redis cache client + cache-aside helpers
   - Middleware stack (gzip, timeout, request ID, recovery, secure headers)

3. **Feature Modularization**
   - Feature directories (`internal/campaign`, `internal/creative`, etc.) with cohesive model/repo/service/handler units

4. **Observability & Ops**
   - Prometheus metrics, OpenTelemetry tracing
   - Health/readiness endpoints, graceful shutdown polish
   - Background worker/CronJob separation

5. **Kubernetes & Deployment**
   - Hardened Dockerfile, readiness/liveness probes
   - Worker deployments + CronJobs for schedulers

Each milestone entry should capture **what changed**, **why it helps performance/maintainability**, and **any follow-ups** needed.
