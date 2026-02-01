## Performance Work Notes (2026-01-16)

### Findings
- Metrics path: per-tick latency slice copy + sort; replaced with histogram to cut alloc/CPU.
- WebSocket fanout: per-client marshal + concurrent writes; switched to typed payload + single marshal + serialized writes.
- Data plane: TCP/UDP loops doing per-iteration deadlines and buffer alloc; added pooling and reduced deadline churn.
- Manager metrics broadcast: removed per-tick map copy.

### Decisions
- Fixed-bucket latency histogram (1ms buckets, 2s window) to avoid O(n log n) percentiles.
- Single pprof server on `PPROF_ADDR` behind `PPROF_ENABLED`.
- New local load tool `cmd/loadtest` for TCP/UDP/WS pressure.

### New Docs/Targets
- `PERFORMANCE.md` for profiling + load scenarios.
- `Makefile` targets: `loadtest`, `perf-bench`, `perf-smoke`.

## Codebase Analysis Improvements (2026-01-16)

### Findings
- Manager cleanup deadlock risk from nested `m.mu` lock; added locked helper.
- Global rate limiter refill integer division starvation for low rates.
- Origin wildcard matching compared full Origin string; host parsing needed.
- Static web root depended on working dir; added `WEB_ROOT`.
- Logger dropped non-string/non-int values.
- Client latency sort O(n^2); replaced with `sort.Slice`.
- Collector per-call alloc for histogram buckets; pooled slices.
- Dockerfile `RATE_LIMIT_PER_IP` default mismatched README/config.

### Decisions
- Use token bucket refill with float math (per-minute rate).
- Allow `WEB_ROOT` env to pin static assets.
- CI addition deferred; repo not in git.

## Docs + Deploy Updates (2026-01-16)

### Findings
- Compose files missing runtime env parity (rate limits, proxy trust, web root).
- Prod deploy script missing proxy/cors env propagation.

### Decisions
- Add `WEB_ROOT` and rate-limit envs to all compose variants.
- Deploy script now passes `ALLOWED_ORIGINS`, `PUBLIC_HOST`, `TRUST_PROXY_HEADERS`, `TRUSTED_PROXY_CIDRS`, `WEB_ROOT`.

## Frontend Modal Polish (2026-01-16)

### Findings
- Settings modal labels not associated with inputs; focus/AT naming weak.
- Dialog overlay duplicated between full-screen `.modal` and `::backdrop`.
- Modal missing focus-visible styles and scroll/height constraints.

### Decisions
- Add `for`/`aria-labelledby`/`aria-describedby` to modal markup.
- Style `<dialog>` as box; move overlay to `::backdrop`.
- Add modal motion, focus-visible rings, reduced-motion guard.

## CI + GHCR Deployment (2026-01-18)

### Findings
- GHCR publish + SSH deploy fits private server requirement without exposing secrets in repo.
- Existing Docker compose uses local build; needed GHCR-specific compose file.

### Decisions
- Add GitHub Actions workflow to test, build, push GHCR image, then optional SSH deploy.
- Add `docker-compose.ghcr.yaml` and document required secrets in `DEPLOYMENT.md`.

## Release Pipeline (2026-01-18)

### Findings
- SemVer tagging needed for both binaries and GHCR images.
- Client version string was hardcoded and needed build-time injection.

### Decisions
- Add release workflow for tags `v*.*.*` to publish binaries + semver-tagged images.
- Switch CLI version to build-time variable (`main.version`).

## CI Pipeline Optimization (2026-01-18)

### Findings
- CI only on `main` push; no PR signal.
- Docker builds uncached; `latest` updated on main.
- Docker build context included non-build assets; no `.dockerignore`.
- E2E tests not gated for PR fast path.

### Decisions
- Split CI jobs: changes detection, checks, build/push, deploy.
- Use `edge` + `sha` tags on main; `latest` only on releases.
- Add Buildx cache, `.dockerignore`, and BuildKit cache mounts.
- Gate e2e via `testing.Short()`; add nightly confidence workflow.
- Pin GitHub Actions to SHAs; add dependabot + GHCR cleanup job.

## CLI HTTP Streaming Support (2026-01-27)

### Findings
- CLI used stream start + raw TCP/UDP/QUIC only; no HTTP streaming path parity with web UI.

### Decisions
- Add HTTP protocol option in CLI using `/api/v1/download`, `/api/v1/upload`, `/api/v1/ping`.
- Add `--chunk-size` and config/env support for HTTP chunk size.

## Version Endpoint (2026-02-01)

### Actions
- Added `/api/v1/version` returning server build version.

## Playwright E2E Setup (2026-02-01)

### Actions
- Added Playwright/Bun config and basic UI tests.
- Verified local run via `bunx playwright test`.

## Reliability Fixes (2026-02-01)

### Actions
- Per-IP rate limiter refill uses float math to avoid low-rate starvation.
- Speedtest download now handles random-data generation errors.
- HTTPS pages upgrade server health checks to avoid mixed content.
- CI checkout fetches tags to derive semver+sha on edge builds.

## Web Mixed Content Fix (2026-02-01)

### Actions
- Server `/api/v1/servers` now reports scheme based on request headers.
- Web upload/download streams fail fast on network errors to reduce spam.

## Dead Code Analysis (2026-01-24)

### Findings
- `pkg/errors/errors.go:52` - `ErrConnectionFailed`: exported, never used.
- `pkg/errors/errors.go:75` - `IsContextError`: exported, never used.
- `pkg/types/rtt.go:87` - `MeasureRTT`: exported, only used internally by `MeasureBaselineRTT`.
- `internal/metrics/calculator.go:10` - `CalculateLatency`: exported, only used in tests (production uses `CalculateLatencyFromHistogram`).

### Actions
- Removed `ErrConnectionFailed`/`IsContextError`.
- Renamed `MeasureRTT` to `measureRTT`.
- Kept `CalculateLatency` for test usage.

## Test Relocation (2026-01-24)

### Actions
- Moved unit tests/benchmarks into `test/unit/**`.
- Adjusted router/origin tests to assert CORS behavior via middleware.
- Adjusted websocket origin tests to use `httptest` + websocket dial.
- Added `logging.FormatValue` for external tests; benchmarks use local structs/constants.

## Rules + Skills Refresh (2026-01-24)

### Findings
- Rules/skills verbose; condensed to agentic best-practice checklist.

### Decisions
- Keep minimal core: workflow, safety, testing, tool order, skill usage.

## Cleanup Pass (2026-01-24)

### Actions
- Removed duplicate `StartStream` branches in handler.
- Dropped unused `Collector.GetSnapshot` wrapper.
- Centralized host normalization in API handlers.
- Removed unused connection state types from `pkg/types`.

## Dependabot Fix (2026-01-24)

### Actions
- Updated `golang.org/x/crypto` to `v0.45.0` (CVE-2025-47914, CVE-2025-58181).
- Updated `golang.org/x/net` to `v0.47.0` via transitive upgrade.

## Workflow Fix (2026-01-26)

### Actions
- Removed invalid input combo in `ghcr-cleanup` (drop `num-old-versions-to-delete` with `min-versions-to-keep`).

## Deep Analysis Improvements (2026-01-24)

### Actions
- Rate limiter cleanup reads now lock per-IP state before TTL check.
- Speedtest upload now handles body read errors; added unit test.
- Speedtest random data init failure now falls back to per-request random.
- WebSocket ping loop now stoppable; server closes cleanly on shutdown.
- Registry service cleanup loop now waits on stop.
