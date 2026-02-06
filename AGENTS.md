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

## Speed Test Accuracy Improvements (2026-02-05)

### Findings
- Live speed display used raw 200ms interval values (noisy, erratic numbers).
- Latency measurement included DNS/TLS cold-start pings inflating median and jitter.
- No loaded latency / bufferbloat detection — only idle latency measured.
- Fixed grace periods (1.5s download, 3s upload) wrong for both slow and fast connections.
- Fixed 1.06 overhead factor assumed HTTP/1.1; HTTP/2 framing overhead is negligible.
- Stream count options capped at 16; no guidance for gigabit+ links.

### Actions
- **EWMA smoothing**: live speed display uses exponentially weighted moving average (alpha=0.3, ~1s window). Final result still uses raw `totalBytes / elapsed`.
- **Latency outlier filtering**: 24 samples (up from 20), discard first 2 warm-up pings, IQR-based outlier filter before computing median/jitter. `filterOutliersIQR()` shared by idle and loaded latency.
- **Loaded latency / bufferbloat**: background ping loop (500ms interval) during download and upload phases via `startLoadedLatencyProbe()`. Results displayed as "Loaded Latency" + letter grade (A+ through F) based on latency increase under load.
- **Dynamic warm-up**: `createWarmUpDetector()` tracks 500ms rolling throughput windows. Grace ends when last 3 windows vary < 15%. Hard cap at `min(duration * 0.3, 5s)`. Replaces fixed 1.5s/3s grace periods.
- **Protocol-aware overhead**: `detectOverheadFactor()` uses Resource Timing API `nextHopProtocol`. Returns 1.0 for HTTP/2+, 1.02 for HTTP/1.1 (was hardcoded 1.06).
- **Stream count options**: added 32-stream option; updated capacity estimates in dropdown.
- HTML: added "Idle Latency", "Loaded Latency", and "Bufferbloat" result fields.

## Capacity-Derived Concurrency Limits (2026-02-05)

### Findings
- `NewSpeedTestHandler(20)` was hardcoded; 32-stream users would get 503s.
- `MAX_STREAMS` validation capped at 16; incompatible with new 32-stream UI option.
- `MAX_CONCURRENT_TESTS` and `MAX_CONCURRENT_PER_IP` only affect TCP/UDP stream manager, not HTTP speed tests — misleading in docs.

### Actions
- Added `Config.MaxConcurrentHTTP()` → derives from `CapacityGbps`: `max(capacity * 8, 50)`. At 1 Gbps → 50; at 25 Gbps → 200.
- `NewRouter` now accepts `*config.Config`; passes `MaxConcurrentHTTP()` to `NewSpeedTestHandler`.
- Raised `MaxStreams` validation cap from 16 to 64; default from 16 to 32.
- Removed `MAX_CONCURRENT_TESTS`, `MAX_CONCURRENT_PER_IP`, `MAX_STREAMS` from Dockerfile defaults and compose files (good defaults via config).
- Removed from README env var table (still work as overrides, just not primary config surface).
- Updated DEPLOYMENT.md and API.md to reference `CAPACITY_GBPS` for scaling guidance.
- Added config unit tests for `MaxConcurrentHTTP()` and `MaxStreams` validation.

## SQLite Result Storage + Share Button (2026-02-05)

### Findings
- No persistence for test results; users couldn't share or revisit results.
- Docker builds use `CGO_ENABLED=0` — need pure Go SQLite driver.

### Decisions
- `modernc.org/sqlite` (pure Go, no CGO) for cross-compilation compatibility.
- 8-char base62 IDs via `crypto/rand` for short, URL-safe result links.
- 90-day retention + configurable max count (default 10,000) with hourly cleanup.
- Results saved automatically on test completion; share button copies URL to clipboard.
- Shared results page (`/results/{id}`) serves static HTML that fetches data via API.

### Actions
- Created `internal/results/store.go` — SQLite storage with migration, save, get, cleanup loop.
- Created `internal/results/handler.go` — POST/GET handlers with validation.
- Created `web/results.html` — shared result viewer matching existing UI style.
- Added `DataDir` + `MaxStoredResults` to config with env support (`DATA_DIR`, `MAX_STORED_RESULTS`).
- Wired results store + handler into router and server main; cleanup on shutdown.
- Added share button to `web/index.html`; auto-save logic + clipboard share in `web/app.js`.
- Styled share button in `web/style.css`.
- Added `DATA_DIR` env + `/app/data` volume to Dockerfile and all compose files.
- Added `data/` and `*.db` to `.gitignore`.
- Added unit tests: store CRUD, trim-to-max, handler validation, round-trip.
- Full test suite passes with race detection.

## Improvement Round 1 (2026-02-05)

### Findings
- `validateConfig` hardcoded stream limit to 16; config allows up to 64.
- JSON encode errors silently ignored in results handler and speedtest handlers.
- Config `Validate()` missing checks for `DataDir` and `MaxStoredResults`.
- SQLite store missing `db.Ping()` health check on open.
- Bufferbloat grade calculation duplicated in `showResults()` and `saveAndEnableShare()`.
- Missing OG/description meta tags on both HTML pages.
- Share button missing `:focus-visible` styles; external links missing `rel="noopener"`.
- Invalid `closed` attribute on `<dialog>` element.
- Results page error/loading views using inline styles.
- Unsafe type assertions (`sync.Pool.Get()`) in collector and client engine could panic.

### Actions
- `validateConfig` now uses `h.config.MaxStreams` instead of hardcoded 16.
- Added `json.Encode` error logging in results handler (`Save`, `Get`) and speedtest (`Upload`, `Ping`).
- Added `DataDir` and `MaxStoredResults` validation to `config.Validate()`.
- Added `db.Ping()` health check in `results.New()`.
- Extracted `computeBufferbloatGrade()` utility function in `app.js`; used in both `showResults` and `saveAndEnableShare`.
- Added `<meta name="description">`, `og:title`, `og:description`, `og:type` to `index.html` and `results.html`.
- Added `:focus-visible` styles for `.share-btn` and `.restart-btn`; `rel="noopener noreferrer"` on external links.
- Removed invalid `closed` attribute from `<dialog>`.
- Moved results page inline styles to CSS classes (`.error-view`, `.error-code`, `.error-message`, `.loading-text`, `.btn-link`).
- Added ARIA `role="status" aria-live="polite"` to results loading view.
- Fixed unsafe `sync.Pool` type assertions in `collector.go` and `engine.go` — safe assertion with fallback allocation.

## Improvement Round 2 (2026-02-05)

### Findings
- CLI stream validation hardcoded to max 16; server allows 64.
- `API.md` missing `/api/v1/results` endpoint docs.
- `README.md` missing `DATA_DIR` and `MAX_STORED_RESULTS` env vars.
- `DEPLOYMENT.md` systemd example missing `DATA_DIR`.
- `download.html` missing meta tags and `rel="noopener"` on external links.
- Missing unit tests for config `DataDir`/`MaxStoredResults` validation and handler stream limit.

### Actions
- Updated CLI stream limit from 16 to 64 in `cli.go` (flags, validation, help text, env docs) and `config.go`.
- Added full saved results API section to `API.md` (POST, GET, page route).
- Added `DATA_DIR` and `MAX_STORED_RESULTS` to `README.md` env table.
- Added `DATA_DIR` to `DEPLOYMENT.md` systemd service example.
- Added meta tags (`description`, `og:title`, `og:description`, `og:type`) and `rel="noopener noreferrer"` to `download.html`.
- Added `TestDataDirValidation`, `TestMaxStoredResultsValidation` to config tests.
- Added `TestStartStreamRespectsMaxStreams` to handler tests — verifies dynamic stream limit from config.

## Improvement Round 3 (2026-02-05)

### Findings
- Dynamically created `<a>` elements in `download.html` missing `rel="noopener noreferrer"`.
- Results store `Save()` didn't handle unique ID collisions; INSERT would fail on rare duplicates.
- `validateConfig` fallback `maxStreams` was still 16 instead of 32 (matching default config).
- Results handler `Save()` missing `Content-Type: application/json` validation.
- Stale "1-16" stream references in `API.md`, `README.md`, and `ARCHITECTURE.md`.

### Actions
- Added `rel="noopener noreferrer"` to all dynamically created download links (3 locations).
- Added retry loop (max 5 attempts) to `Store.Save()` on UNIQUE constraint violations.
- Updated `maxStreams` fallback from 16 to 32 in `validateConfig`.
- Added `Content-Type` validation to results handler `Save()` — returns 415 on non-JSON.
- Updated stream limit references from "1-16" to "1-64" in `API.md`, `README.md`, `ARCHITECTURE.md`.

## Download Page Redesign (2026-02-05)

### Findings
- Download page was barebones — three cards with architecture links, no version info, no file sizes, no platform detection, no install instructions.

### Actions
- Redesigned `web/download.html` with:
  - Auto-detected platform recommendation (OS + architecture via WebGL renderer for Apple Silicon).
  - Prominent primary download button with file size.
  - Alternate architecture link.
  - Version tag + release date from GitHub API.
  - Quick Install section with tabbed commands (curl/docker for Linux/macOS, powershell/docker for Windows).
  - All Platforms grid with file sizes per asset.
  - Docker section with copy-to-clipboard.
- Updated `web/style.css` with new download page component styles (recommended card, primary button, install tabs, code blocks, asset rows, copy buttons, docker card).

## Embedded Web Assets (2026-02-05)

### Findings
- Web assets (~50KB) were served from disk via `http.Dir`; required bundling `web/` in releases and Docker images.
- `WEB_ROOT` env var defaulted to `./web`, causing "directory not found" errors in single-binary deployments.

### Decisions
- Use `//go:embed` in `web/embed.go` to bake all static assets into the binary.
- Router defaults to embedded `http.FS(web.Assets)`; `WEB_ROOT` env overrides to disk for dev.
- `results.html` served via `http.ServeContent` from the resolved FS.

### Actions
- Created `web/embed.go` with `//go:embed *.html *.css *.js`.
- Updated `internal/api/router.go`: replaced `http.Dir(webRoot)` with `resolveWebFS()` (embedded default, disk override).
- Updated `internal/config/config.go`: `WebRoot` default now empty (embedded); removed empty-string validation.
- Removed `COPY web/ /app/web/` from `docker/Dockerfile` (web/ still copied to build stage for embed).
- Removed `cp -R web "$out/web"` from `.github/workflows/release.yml`.
- Removed `WEB_ROOT` env from all Docker Compose files.
- Updated `README.md`, `DEPLOYMENT.md`, `web/download.html` to reflect embedded assets.
- Removed `WEB_ROOT=./web` from `playwright.config.js`.

## Improvement Round 4 (2026-02-06)

### Findings
- Unsafe `sync.Pool` type assertions in `engine.go` (bidirectional) and `http_engine.go` (download) — panic risk.
- UDP sender `copy` panics if `UDPBufferSize > randomDataSize` — no bounds check.
- Registry handler: API key compared with `==` (timing attack); no body size limits; all `json.Encode` return values silently dropped.
- TOCTOU race on `activeDownloads`/`activeUploads` counter — check-then-increment allows exceeding max by 1.
- Router `results.html` handler: `f.Stat()` error unchecked — nil stat dereference panic.
- `HealthCheck` `w.Write` error unchecked.
- Mixed `log.Printf` + `logging.Error` in server shutdown.
- `JSONFormatter` drops encode errors.
- `randomData` init failure logged to no-one.
- Dead code: RTC rate-limit skip paths, streamID fallback, unused `saveConfigFile`.
- Deprecated `version: '3.8'` in compose files.

### Actions
- Safe pool assertions with fallback in `engine.go` (bidirectional goroutines) and `http_engine.go`.
- Added bounds check in `udpSender` for `UDPBufferSize > randomDataSize`.
- Registry: `subtle.ConstantTimeCompare` for API key auth; `MaxBytesReader(64KB)` on register/update; all `json.Encode` calls checked+logged.
- Atomically increment-then-check for download/upload concurrency (eliminates TOCTOU).
- Router: `f.Stat()` error checked, returns 404 on failure.
- `HealthCheck` write error logged.
- Removed duplicate `log.Printf` from server shutdown; removed unused `log` import.
- `JSONFormatter.FormatComplete` now logs encode errors.
- `randomData` init failure now logged with warning.
- Removed dead `rtc/offer`, `rtc/ice` rate-limit skip paths.
- Removed dead `streamID` fallback in `StartStream`.
- Removed unused `saveConfigFile` from client config.
- Removed deprecated `version: '3.8'` from `docker-compose.yaml`, `docker-compose.traefik.yaml`, `docker-compose.multi.yaml`.

### Deferred
- `ServerInfo` dedup across `api` and `registry` packages — requires shared types package, skip for now.
- CLI HTTP engine hardcoded overhead/grace — separate improvement scope.
- Registry API documentation in `API.md` — follow-up.
- UDP single-threaded read loop — architectural change, separate scope.

## Improvement Round 5 (2026-02-06)

### Findings
- Alpine 3.19 in Dockerfile reached EOL (Nov 2025).
- Makefile `perf-bench` target pointed to old paths (`./internal/*`) after test relocation.
- SQLite `MaxOpenConns(1)` serializes all DB access; WAL mode supports concurrent readers.
- Stats goroutine in `perf.go` had no stop channel — leaked on server shutdown.
- `LoadFromEnv` silently ignored invalid numeric env vars (TCP_TEST_PORT, CAPACITY_GBPS, rate limits, etc.).

### Actions
- Bumped Alpine from 3.19 to 3.21 in Dockerfile (both server and client stages).
- Fixed Makefile `perf-bench` paths to `./test/unit/*`; changed `-run Test` to `-run ^$$` to skip tests.
- Increased SQLite `MaxOpenConns` to 3, `MaxIdleConns` to 2 for WAL concurrent reads.
- `startRuntimeStatsLogger` now returns a stop function; called during shutdown.
- `LoadFromEnv` now returns errors for invalid TCP_TEST_PORT, UDP_TEST_PORT, CAPACITY_GBPS, MAX_CONCURRENT_TESTS, MAX_STREAMS, RATE_LIMIT_PER_IP, GLOBAL_RATE_LIMIT, MAX_CONCURRENT_PER_IP, PERF_STATS_INTERVAL, MAX_STORED_RESULTS.
- Updated config tests to expect errors for invalid env values.
