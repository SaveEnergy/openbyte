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

## API Body Limits Pass (2026-01-24)

### Actions
- Added JSON body size limits for stream start/metrics/complete.
- Added oversized body regression test for stream start.
- Updated API download chunk defaults to match implementation.

## Web Server Selection Hardening (2026-01-24)

### Actions
- Added safe fetch timeout helper for server health checks.
- Avoided CORS mode for same-origin health requests.

## Network Info Display (2026-01-24)

### Actions
- Show connection type with estimate labels when browser only reports effectiveType.

## Network Info Display Update (2026-02-02)

### Actions
- Removed connection type display from the web UI due to accuracy concerns.

## QUIC Removal (2026-02-02)

### Actions
- Removed QUIC server/client support and related config/docs to reduce surface area.

## Unified Binary Refactor (2026-02-02)

### Actions
- Merged server/client into a single `openbyte` binary with subcommands.

## Defaults + Validation Alignment (2026-01-24)

### Actions
- Align web defaults with CLI and API stream defaults.
- Add client env var docs and extend env parsing for timeout/warmup.
- Enforce UUID stream IDs at routing layer with test coverage.

## Web Download Retry (2026-01-24)

### Actions
- Retry HTTP download streams with smaller chunk size on network errors.

### Follow-up
- Added multi-step chunk fallback (1MB -> 256KB -> 64KB).

## Playwright CI Integration (2026-01-24)

### Actions
- Added Playwright UI test target and CI execution via Bun.

## Deep Analysis Fixes (2026-02-02)

### Actions
- Added mutex to latency histogram; race-safe Record/Reset/CopyTo.
- Manager cleanup/broadcast avoid long lock holds; channel sends outside lock.
- Stream server uses safe buffer pool gets; echo write handles errors.
- WebSocket metrics marshal now logs errors.
- Registry client logs response close failures.
- Config validates global rate limit + max test duration; loads GLOBAL_RATE_LIMIT env.
- GHCR compose exposes 8080; Traefik GHCR env parity with standard compose.
- Added config unit tests for env parsing + validation.

## Code Quality Pass (2026-02-05)

### Findings
- `handleEcho` indentation broken after echo write fix.
- Redundant `n < 1` after `n == 0` in `handleUDP`.
- `ActiveCount()` built full slice just to count.
- API `validateConfig` hardcoded 300s max duration; ignored `config.MaxTestDuration`.
- `respondJSON` silently dropped `json.Encode` errors.
- `elapsed.Seconds() == 0` float comparison fragile in speedtest upload.
- `originHost`/`stripPort` duplicated between `api/router.go` and `websocket/server.go`.
- Speedtest Ping/Upload used manual JSON string construction; no escaping safety.
- Registry client didn't drain response body; prevented HTTP connection reuse.
- `SnapshotLatencyStats` returned 7 unnamed values; error-prone call sites.
- `isPrivateIP` re-parsed CIDR strings on every call.

### Actions
- Fixed indentation in `handleEcho`; removed dead `n < 1` check.
- `ActiveCount()` counts under RLock without slice allocation.
- `validateConfig` reads `config.MaxTestDuration` for upper bound.
- `respondJSON` logs encode errors via `logging.Warn`.
- Upload elapsed check uses `elapsed <= 0`.
- Extracted `StripHostPort`/`OriginHost` to `pkg/types/network.go`; both packages use shared helpers.
- Speedtest Ping/Upload responses use `json.NewEncoder` instead of string concat.
- Registry client drains response body before close via `drainAndClose` helper.
- `SnapshotLatencyStats` returns named `LatencySnapshot` struct.
- `isPrivateIP` uses package-level pre-parsed `[]*net.IPNet`.
- Added tests: `StripHostPort`/`OriginHost`, max duration validation, concurrent Record+GetMetrics race test.
- `SnapshotLatencyStats` returns named `LatencySnapshot` struct (was 7 unnamed values).
- `isPrivateIP` pre-parses CIDR blocks at package init (was re-parsing per call).
- Fixed data race in client `runWarmUp`: shared buffer across goroutines → per-goroutine buffer.
- Client `cancelStream`/`completeStream` now drain+close response bodies for connection reuse.
- `completeStream` handles `json.Marshal`/`http.NewRequest` errors.
- Removed unnecessary `string(jsonData)` copy in `startStream`.
- `measureHTTPPing` HTTP client now has 10s timeout.

## Proxy + Streaming Fixes (2026-02-05)

### Findings
- `GetServers` appended internal container port (`:8080`) to `api_endpoint` even behind reverse proxy, making health check URLs unreachable (e.g. `https://host:8080/health` instead of `https://host/health`).
- Default `WriteTimeout` (15s) killed streaming download connections mid-transfer, causing `ERR_HTTP2_PROTOCOL_ERROR` through Traefik.

### Actions
- `GetServers` detects proxied requests via `X-Forwarded-Proto`/`X-Forwarded-For`/`PublicHost`; skips internal port in `api_endpoint` when proxied.
- Changed default `WriteTimeout` to 0 (disabled); streaming endpoints manage own duration; `IdleTimeout` + `ReadTimeout` still protect against stuck connections.

## Cancel + Restart 503 Fix (2026-02-05)

### Findings
- `fetchWithTimeout` created its own `AbortController` and **replaced** the caller's signal, so `cancelTest() → state.abortController.abort()` never reached in-flight downloads.
- Server-side download handlers kept running for full duration after client cancel; filled `maxConcurrent` (20) slots.
- Next test start got 503 on every download request; retry logic treated 503 same as network error, retrying across all chunk sizes (3 sizes × 3 retries × 4 streams ≈ 36 failing requests).
- Upload path had same `fetchWithTimeout` signal override but less severe (while loop checks `isRunning`).

### Actions
- `fetchWithTimeout` now chains caller's abort signal with its timeout controller via `addEventListener('abort', ...)`.
- `downloadStream` throws specific error (`.status = 503`) on server overload instead of returning false.
- Download retry loop checks `state.isRunning` before each retry; exits on 503 with brief backoff instead of retrying.
- Upload loop breaks on 503 with brief backoff.
- Added `TestDownloadConcurrentLimitAndRelease`: fills maxConcurrent slots → verifies 503 → cancels all → verifies new download succeeds.

## IPv4/IPv6 Detection (2026-02-05)

### Findings
- Server + Traefik properly report client IPs via X-Forwarded-For; both `curl -4` and `curl -6` confirm end-to-end.
- Browser uses Happy Eyeballs — can't control address family from `fetch()`.
- `measureLatency()` discarded ping response bodies; IP info only came from stale page-load probe.

### Actions
- Separate IPv4 and IPv6 address fields in network info UI (replaced single "Client IP").
- `detectNetworkInfo()` runs three parallel probes: main ping + `v4.` subdomain (A-only) + `v6.` subdomain (AAAA-only).
- `measureLatency()` parses first ping response to capture fresh IP during test.
- Updated server `.env` to include `v4.speed.sqrtops.de` and `v6.speed.sqrtops.de` in Traefik host rule.

### DNS Requirements
- Add **A-only** record for `v4.<domain>` → server IPv4 address (no AAAA record).
- Add **AAAA-only** record for `v6.<domain>` → server IPv6 address (no A record).
- Traefik auto-issues Let's Encrypt certs once DNS propagates.
