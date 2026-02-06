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

## Improvement Round 6 — Analysis (2026-02-06)

### Frontend Findings
- `app.js:674-716`: Latency ping response bodies not consumed when `capturedIP` is true — delays HTTP/2 connection reuse.
- `app.js:458-482`: `onCustomServerChange()` doesn't reset `apiBase` when input cleared — stale custom server stays active.
- `app.js:1296-1313`: Success toast reuses `⚠` warning icon — should toggle to `✓`.
- `style.css:919`: `border: none` immediately overridden by line 925 — dead CSS.
- `download.html:146`: `navigator.platform` deprecated; may return empty in modern browsers.
- `app.js:143-156`: `loadSettings` no validation — corrupted localStorage values pass through unchecked.
- `style.css:1249-1253`: `prefers-reduced-motion` only disables modal animation; `fadeIn`/`slideUp`/`pulse` still run.

### Test Quality Findings
- `store_test.go:86-138`: `TestStoreTrimToMax` inserts 5 results with near-identical timestamps — `ORDER BY created_at DESC` tie-break depends on SQLite rowid behavior.
- Missing upload 503 concurrency limit test (download has one).
- `speedtest_test.go:103`: `time.Sleep(50ms)` for goroutine sync — flaky under CI load.
- Missing tests: download duration/chunk clamping, results `Cache-Control` header.

### API Consistency Findings
- 12+ locations use `http.Error()` (plaintext) instead of JSON error format documented in `API.md` — results handler, speedtest 503s, registry auth errors.
- Download handler hardcodes duration max to 60s, ignores `config.MaxTestDuration`.
- Invalid download params silently clamped (stream API returns 400).

### Build/CI Findings
- Tag push triggers both `ci.yml` and `release.yml` Docker builds — duplicate work + potential race on image tags.
- CI edge images are amd64-only; release builds both amd64+arm64.
- Playwright browser installations not cached (~200MB per run).

### Go Code Quality Findings
- `config.go:261-272`: `REGISTRY_INTERVAL`/`REGISTRY_SERVER_TTL` silently ignore invalid env values (inconsistent with round 5 fail-fast pattern).
- `manager.go`: Pending streams never cleaned up — accumulate indefinitely if `StartStream` not called.
- `config.go`: `PORT` not validated as numeric — `PORT=banana` causes confusing runtime error.
- `handlers.go:130,463`: Error matching uses type assertion instead of `errors.As` — breaks if errors are wrapped.
- `logging.Init` uses `sync.Once` — prevents log level reconfiguration after first call.
- `ratelimit.go:95-117`: `allowIP` holds write lock during periodic cleanup — blocks all API requests.

### Documentation Findings
- `DEPLOYMENT.md:19,185`: Quick deploy copies `web/` folder — unnecessary since embed refactor.
- `API.md:386`: Unclosed code fence in Ping section breaks markdown.
- `README.md:60`: "Pre-test warm-up phase (2s default)" outdated — web uses dynamic warm-up.

### Performance Findings
- `logging/logger.go:115-128`: `formatFields` uses string concatenation in loop — allocation per log line.
- `manager.go:233-247`: `ActiveCount()` scans all streams on every `/servers` request.
- `results/store.go:200-210`: `generateID` makes 8 syscalls per ID — could batch entropy.

### Actions Taken
- **A1**: Replaced all `http.Error` with JSON `{"error":"..."}` in `results/handler.go` (7 calls) and `speedtest.go` (4 calls).
- **F2**: `onCustomServerChange` now resets `apiBase`/`selectedServer` when input is cleared.
- **F3**: Toast icon toggles between `⚠` (error) and `✓` (success).
- **G1**: `REGISTRY_INTERVAL`/`REGISTRY_SERVER_TTL` now fail-fast on invalid values (consistent with round 5 pattern).
- **G3**: `PORT` env validated as numeric.
- **C1**: Removed tag trigger from CI `build-push`/`deploy` — tags are handled exclusively by `release.yml`.
- **D1**: Removed stale `web/` copy instructions from `DEPLOYMENT.md` (quick deploy + manual deploy sections).
- **D3**: Fixed README warm-up description (now mentions dynamic warm-up).
- **P1**: `formatFields` uses `strings.Builder` instead of `+` concatenation.

### Deferred (carried forward)
- `ServerInfo` dedup across `api` and `registry` packages.
- CLI HTTP engine hardcoded overhead/grace.
- Registry API documentation in `API.md`.
- UDP single-threaded read loop.
- Flaky concurrency test sleep (T3).
- Latency ping response body drain (F1).
- `navigator.platform` deprecation (F5).
- `prefers-reduced-motion` coverage (F7).

## Improvement Round 7 (2026-02-06)

### Findings
- `handlers.go:130,463`: `StreamError` type assertion used `err.(*errors.StreamError)` instead of `errors.As` — breaks if errors are wrapped.
- `handlers.go:443-449`: `respondJSONBodyError` used `http.Error` (plaintext) — inconsistent with JSON API format established in R6.
- `speedtest.go:47`: `respondSpeedtestError` ignored `json.Encode` return value.
- `speedtest.go:60`: Download `MaxTestDuration` hardcoded to 60s — ignored `config.MaxTestDuration`.
- `manager.go:283-299`: Pending streams never cleaned up — accumulate indefinitely if `StartStream` not called, consuming `maxStreams` slots.
- `registry/handler.go`: All error responses (6 locations) used `http.Error` (plaintext) — inconsistent with JSON API format.
- `registry/client.go:66,199`: Error log messages dropped actual error values — "Initial registration failed" with no context.
- `registry/client.go:99,136`: `register`/`heartbeat` used `http.NewRequest` without context — no cancellation propagation.
- `registry/client.go:163,178`: `deregister` error logs dropped error values.
- `rtt.go:32`: `r.samples = r.samples[1:]` reslice didn't free backing array — slow memory leak in long-lived `RTTCollector`.
- `config.go:119-124,295-297`: `PORT` validated as numeric but not range-checked — `PORT=99999` passed validation.
- `router.go:250-265`: `responseWriter` wrapper implemented `Hijack` but not `http.Flusher` — broke flushing for any logging-middleware-wrapped response.
- `app.js:143-156`: `loadSettings` didn't validate parsed values — corrupted `localStorage` could set `NaN` duration/streams.
- `app.js:354-366`: `selectFastestServer` didn't consume health check response body on success — delayed HTTP/2 connection reuse.
- `app.js:509`: `checkServer` skipped `!res.ok` responses without consuming body.

### Actions
- `handlers.go`: Replaced `err.(*errors.StreamError)` with `errors.As` in `CreateStream` error handler and `respondError`.
- `handlers.go`: `respondJSONBodyError` now returns JSON `{"error":"..."}` via `respondJSON` instead of `http.Error`.
- `speedtest.go`: `respondSpeedtestError` now logs `json.Encode` errors.
- `speedtest.go`: `NewSpeedTestHandler` accepts `maxDurationSec` param; `Download` uses it instead of hardcoded 60.
- `router.go`: `NewRouter` passes `MaxTestDuration` from config to `NewSpeedTestHandler`.
- `router.go`: `responseWriter` now implements `http.Flusher` (delegates to underlying writer).
- `manager.go`: `cleanup()` now handles `StreamStatusPending` — fails and removes pending streams older than 30s.
- `registry/handler.go`: Added `respondRegistryError` helper; replaced all 6 `http.Error` calls with JSON responses.
- `registry/client.go`: `register`, `heartbeat`, `deregister` error logs now include `logging.Field{Key: "error", Value: err}`.
- `registry/client.go`: `register` and `heartbeat` now use `http.NewRequestWithContext` with 10s timeout.
- `rtt.go`: Replaced slice reslice with proper ring buffer (`head`/`count` indices, fixed-size backing array).
- `config.go`: `Validate()` now checks `PORT` is in range 1-65535.
- `app.js`: `loadSettings` validates parsed `duration`, `streams`, `serverUrl` types before applying.
- `app.js`: `selectFastestServer` consumes health response body via `res.text()` on both ok and unhealthy paths.
- `app.js`: `checkServer` consumes response body on `!res.ok` before continuing.
- Added tests: `TestUploadConcurrentLimitAndRelease`, `TestDownloadRespectsMaxDuration`, `TestHandlerSaveRejectsWrongContentType`, `TestPendingStreamCleanup`, `TestActiveCountMatchesActiveStreams`, `TestConfigValidatePortRange`.
- Updated all `NewSpeedTestHandler` call sites (6 locations) with new `maxDurationSec` parameter.
- Full test suite passes with race detection.

### Deferred (carried forward)
- `ServerInfo` dedup across `api` and `registry` packages.
- CLI HTTP engine hardcoded overhead/grace.
- Registry API documentation in `API.md`.
- UDP single-threaded read loop.
- Flaky concurrency test sleep (T3).
- `navigator.platform` deprecation (F5).
- `registry/client.go:222`: `buildServerInfo` hardcodes `http://` scheme — wrong behind TLS termination.
- `websocket/server.go`: `sentStatus` map never cleaned for clientless streams.
- `results/store.go`: `generateID` makes 8 syscalls per ID — could batch entropy.

## Improvement Round 8 (2026-02-06)

### Findings
- `collector.go:125`: `startTime` read outside `c.mu.RUnlock` — data race with `Reset()`.
- `collector.go:64-69`: `RecordLatency` holds `c.mu.Lock` around `c.latencyHistogram.Record()` — double-mutex contention (histogram has its own lock).
- `ratelimit.go:164`: 429 response used `http.Error` (plaintext) — all other API errors are JSON.
- `router.go:115,155,159`: `HandleWithID` and WS route returned `http.Error` plaintext on API paths.
- `app.js:797-802`: Loaded latency probe never drained ping response body — HTTP/2 stream leak during test.
- `app.js:1064-1069`: Upload `!res.ok` path continued without consuming response body.
- `app.js:168-169`: `saveSettings` used `parseInt` without radix or NaN validation.
- `style.css:1249-1253`: `prefers-reduced-motion` only suppressed modal animation — WCAG violation.
- `style.css:919`: Dead `border: none` immediately overridden by `border: 1px solid`.
- `manager.go:233-247`: `ActiveCount()` O(n) scan over all streams (including retained completed) on every `/servers` request.
- `API.md:318`: Download max duration doc said 60, actual is config-driven (default 300).

### Actions
- `collector.go`: Read `startTime` inside `c.mu.RLock` section — eliminates race with `Reset()`.
- `collector.go`: `RecordLatency` calls `c.latencyHistogram.Record()` *outside* `c.mu.Lock` — eliminates double-locking on hot path.
- `ratelimit.go`: 429 response now returns JSON `{"error":"rate limit exceeded"}`.
- `router.go`: `HandleWithID` and WS route errors now use `respondJSON` for JSON format.
- `app.js`: Loaded latency probe now drains ping response body with `res.text()`.
- `app.js`: Upload `!res.ok` path now drains response body before `continue`/`break`.
- `app.js`: `saveSettings` uses `parseInt(..., 10)` with `Number.isFinite` + `> 0` validation.
- `style.css`: `prefers-reduced-motion` now suppresses *all* animations and transitions.
- `style.css`: Removed dead `border: none` from `.modal`.
- `manager.go`: Added `activeCount int64` field; incremented atomically in `CreateStream`, decremented in `releaseActiveStreamLocked`. `ActiveCount()` is now O(1) via `atomic.LoadInt64`.
- `API.md`: Updated download max duration from 60 to "configurable via MAX_TEST_DURATION, default 300".

### Deferred (carried forward)
- `ServerInfo` dedup across `api` and `registry` packages.
- CLI HTTP engine hardcoded overhead/grace.
- Registry API documentation in `API.md`.
- UDP single-threaded read loop.
- Flaky concurrency test sleep (T3).
- `navigator.platform` deprecation (F5).
- `registry/client.go:222`: `buildServerInfo` hardcodes `http://` scheme — wrong behind TLS termination.
- `websocket/server.go`: `sentStatus` map never cleaned for clientless streams.
- `results/store.go`: `generateID` makes 8 syscalls per ID — could batch entropy.

## Improvement Round 9 — Analysis (2026-02-06)

### HIGH Findings
- **F1**: TCP stream server (`stream/server.go:138,166-352`) accepts unlimited connections — no auth, no concurrency limit, no duration cap. All TCP handlers loop until client disconnects. DoS vector for public deployments.
- **F2**: WebSocket server (`websocket/server.go:67,94`) never calls `SetReadLimit()`. Default is unlimited. Malicious client can send multi-GB message, consuming server memory.
- **F3**: Stream Manager (`manager.go:88`) capacity check uses `len(m.streams)` which includes completed/retained streams (1hr retention). Completed streams block new stream creation even though slots are free.

### MEDIUM Findings
- **F4**: `measureLatency` (`app.js:704-733`) — 22 of 24 pings never drain response body after first sets `capturedIP`. HTTP/2 stream leak during latency phase.
- **F5**: `handlers.go:95,165,242,260` — 4 remaining plaintext `http.Error` on method-not-allowed. R6-R8 fixed all others; these remain inconsistent.
- **F6**: `results.html:155-194` — `renderResult()` assumes API fields are numbers; throws `TypeError` on malformed response with no user feedback.
- **F7**: `fetchWithTimeout` (`app.js:287-289`) — abort listener accumulates on shared signal; hundreds of listeners build up during long tests.
- **F8**: `checkServer` (`app.js:512-536`) — sequential health probes with no per-URL timeout; custom server DNS failure can block UI for 2-3 minutes.
- **F9**: CI (`ci.yml:92-94`) — Playwright browser installations (~200MB) not cached; downloads on every non-PR push.

### LOW Findings
- **F10**: CLI `selectFastestServer` (`cli.go:193`) — `resp.Body.Close()` without drain prevents HTTP connection reuse.
- **F11**: `gorilla/mux v1.8.1` (`go.mod:8`) — archived since Dec 2022. Go 1.25 stdlib router supports path params and method matching natively.
- **F12**: Makefile `perf-smoke` (line 103) — `go run ... &` PID capture gets `go run` PID, not child binary PID; `kill` may leave orphan.

### Actions
- **F1**: Added `activeTCPConns` atomic counter + `maxTCPConns` limit to stream server. TCP connections rejected when limit exceeded. Added `maxConnDur` hard duration cap per connection (MaxTestDuration + 30s grace); context-based deadline closes connection when exceeded.
- **F2**: Added `conn.SetReadLimit(4096)` after WebSocket upgrade — server only reads for disconnect detection, 4KB is generous.
- **F3**: Changed `CreateStream` capacity check from `len(m.streams)` to `atomic.LoadInt64(&m.activeCount)` — completed/retained streams no longer count against capacity.
- **F4**: `measureLatency` now drains response body on all pings after first (added `else { await res.text() }` after IP capture branch).
- **F5**: All 4 method-not-allowed handlers now use `respondJSON` instead of `http.Error` — completes JSON error format migration across entire API surface.
- **F6**: `results.html` `renderResult` wrapped in try/catch with `showError()` fallback; added `safeFixed()` helper for numeric fields; type-checks `d` before rendering.
- **F8**: `checkServer` now uses `fetchWithTimeout(url, {}, 5000)` instead of bare `fetch()` — prevents multi-minute hangs on custom server DNS failures.

### Deferred (carried forward)
- `ServerInfo` dedup across `api` and `registry` packages.
- CLI HTTP engine hardcoded overhead/grace.
- Registry API documentation in `API.md`.
- UDP single-threaded read loop.
- Flaky concurrency test sleep (T3).
- `navigator.platform` deprecation.
- `registry/client.go:222`: `buildServerInfo` hardcodes `http://` scheme.
- `websocket/server.go`: `sentStatus` map never cleaned for clientless streams.
- `results/store.go`: `generateID` makes 8 syscalls per ID.
- `fetchWithTimeout` abort listener accumulation (F7).
- CLI `selectFastestServer` body drain (F10).
- `gorilla/mux` archived — consider stdlib migration (F11).
- Makefile `perf-smoke` PID capture (F12).
- Playwright browser cache in CI (F9).

## Improvement Round 10 — Analysis (2026-02-06)

### HIGH Findings
- **F1**: CORS wildcard subdomain bypass (`router.go:223-228`, `websocket/server.go:318-323`) — `*.example.com` matches `evilexample.com` via bare `HasSuffix`. Missing leading dot check allows any domain ending with the suffix to pass CORS/WebSocket origin validation.
- **F2**: UDP sender goroutine spawn without limit (`stream/server.go:429-438`) — each unique client address spawns an unbounded `udpSender` goroutine. TCP got limits in R9; UDP did not. DoS vector via spoofed/many source addresses.
- **F3**: XFF leftmost-IP trust enables rate limit bypass (`clientip.go:65-84`) — `firstClientIP` returns first public IP from `X-Forwarded-For`. Behind trusted proxy, attacker prepends spoofed IP; function returns attacker-controlled value. Per-IP rate limiting bypassed.

### MEDIUM Findings
- **F4**: Upload handler has no duration cap or context check (`speedtest.go:144-158`) — `io.Copy(io.Discard, r.Body)` blocks until EOF. Download handler has `duration` param + `r.Context().Done()` + deadline. Slow-drip upload holds concurrency slot indefinitely.
- **F5**: UDP client state TOCTOU (`stream/server.go:425-438`) — RLock/check-nil/RUnlock/Lock window allows two packets from same address to both see nil, both create state + spawn sender goroutine. Second overwrites map; first leaks.
- **F6**: UDP sender goroutines not tracked by WaitGroup (`stream/server.go:438`) — `Close()` returns before senders finish; writes to closed `udpConn` possible.
- **F7**: Client TCP/UDP warm-up is a no-op (`engine.go:199-215`) — reads from connections before direction command byte sent. Server blocks waiting for command; all reads time out. Warm-up period wasted.
- **F8**: Client parent context timeout wraps entire lifecycle (`main.go:70`) — default 60s timeout starts before ping/RTT/warmup. HTTP mode consumes ~10s on pings; `-t 55` test gets killed at 60s mark.

### LOW Findings
- **F9**: CORS OPTIONS rejection uses plaintext `http.Error` (`router.go:198`) — R6-R9 migrated all API errors to JSON; this one missed. No functional impact (browser doesn't expose CORS preflight bodies).

### Actions
- **F1**: Fixed CORS wildcard subdomain bypass in `router.go` and `websocket/server.go` — added dot boundary check (`originHostValue == suffix || HasSuffix(originHostValue, "."+suffix)`).
- **F2+F5+F6**: Added `activeUDPSenders`/`maxUDPSenders` atomic limiter. Fixed TOCTOU via double-checked locking after write lock. Added `s.wg.Add(1)` + `defer s.wg.Done()` + `defer atomic.AddInt64(&s.activeUDPSenders, -1)` to `udpSender`.
- **F3**: Replaced `firstClientIP` (leftmost-first) with `rightmostUntrustedIP` — iterates XFF right-to-left, skips trusted proxy CIDRs. Prevents rate limit bypass via spoofed XFF. Removed dead `firstClientIP`.
- **F4**: Upload handler now has duration cap (`maxDurationSec + 30s`), context cancellation check, and non-EOF error detection in read loop.
- **F9**: CORS OPTIONS rejection now returns JSON.
- Updated `TestClientIPResolver_TrustedProxy` for rightmost-untrusted behavior; added `TestClientIPResolver_RightmostUntrusted`.

### Deferred (carried forward)
- `ServerInfo` dedup across `api` and `registry` packages.
- CLI HTTP engine hardcoded overhead/grace.
- Registry API documentation in `API.md`.
- UDP single-threaded read loop.
- Flaky concurrency test sleep (T3).
- `navigator.platform` deprecation.
- `registry/client.go:222`: `buildServerInfo` hardcodes `http://` scheme.
- `websocket/server.go`: `sentStatus` map never cleaned for clientless streams.
- `results/store.go`: `generateID` makes 8 syscalls per ID.
- `fetchWithTimeout` abort listener accumulation.
- CLI `selectFastestServer` body drain.
- `gorilla/mux` archived — stdlib migration candidate.
- Makefile `perf-smoke` PID capture.
- Playwright browser cache in CI.
- Client TCP/UDP warm-up is a no-op (F7).
- Client parent context timeout wraps entire lifecycle (F8).

## Improvement Round 11 (2026-02-06)

### Findings
- CLI context error checks used bare `!=` instead of `errors.Is`; wrapped errors from net package bypassed the check.
- WebSocket metrics read loop blocked on `ReadJSON` inside `select default`; context cancellation delayed up to `readTimeout`.
- No security headers middleware — missing `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy` on all responses.
- CORS wildcard dot-boundary fix (R10) had zero negative test coverage.
- Download endpoint silently clamped invalid `duration`/`chunk` params; inconsistent with stream API returning 400.
- Frontend warmup-to-measurement transition caused EWMA speed display freeze (bytes counter reset without tracking reset).
- `saveAndEnableShare` did not drain response body on `!res.ok` (HTTP/2 stream leak).
- Manager log field "active" used `len(m.streams)` (includes retained) instead of atomic `activeCount`.
- Dead OPTIONS handler on v1 subrouter (unreachable behind CORSMiddleware).
- `X-Content-Type-Options: nosniff` duplicated in speedtest handler (now set by global middleware).
- HTTP engine test context had zero grace period — expired before server-side timer.

### Actions
- Replaced `err != context.DeadlineExceeded` with `errors.Is` (2 call sites in `run.go`).
- WebSocket read loop: spawn goroutine that closes conn on context cancel; removed blocking select.
- Added `SecurityHeadersMiddleware` (nosniff, DENY, strict-origin-when-cross-origin) to router.
- Added `TestRouterRejectsWildcardBypassOrigin` negative test.
- Download handler returns 400 for invalid duration/chunk params; updated test.
- Frontend `onProgress` resets `lastBytes`/`ewmaSpeed` when bytes counter decreases.
- Added response body drain in `saveAndEnableShare`.
- Manager log "active" field now uses `atomic.LoadInt64(&m.activeCount)`.
- Removed dead OPTIONS handler on v1 subrouter.
- Removed duplicate `X-Content-Type-Options` from speedtest download handler.
- Added 10s grace to HTTP engine test context timeout.

## Improvement Round 12 (2026-02-06)

### Findings
- `buildDownloadURL` ignored `url.Parse` error → nil dereference panic on malformed server URL.
- SQLite WAL mode never applied: `modernc.org/sqlite` ignores query-string params; needs explicit PRAGMAs.
- `sql.ErrNoRows` checked with `==` instead of `errors.Is`; wrapped errors cause 500 instead of 404.
- HTTP engine had no `Close()` method; idle connections leaked until GC.
- `io.EOF` compared with `==` in download read loop and upload handler (2 sites).
- Error toast missing `role="alert"` / `aria-live` for screen reader announcement.
- `cancelTest()` contained dead WebSocket/stream code from old TCP/WS path.
- Dockerfile installed `curl` (~3MB) solely for healthcheck; Alpine has `wget` built-in.

### Actions
- `buildDownloadURL` now checks `url.Parse` error; falls back to plain concatenation.
- SQLite `New()` uses explicit `PRAGMA journal_mode=WAL` and `PRAGMA busy_timeout=5000` after open.
- `Store.Get` uses `errors.Is(err, sql.ErrNoRows)`.
- Added `HTTPTestEngine.Close()` calling `CloseIdleConnections()`; called from `runHTTPStream` via defer.
- Replaced `err == io.EOF` / `err != io.EOF` with `errors.Is` in `http_engine.go` and `speedtest.go`.
- Toast div now has `role="alert" aria-live="assertive"`.
- Removed dead `state.ws` / `state.streamId` blocks from `cancelTest()`.
- Dockerfile healthcheck uses `wget -q --spider`; dropped `curl` dependency.

## Improvement Round 13 (2026-02-06)

### Findings
- `engine.go:137` — bare `!=` for context errors; wrapped net errors bypass the check.
- `engine.go` — 5 bare `== io.EOF` / `.(net.Error)` type assertions missed by R11/R12.
- Shutdown order: `wsServer.Close()` before `manager.Stop()` causes writes to closed WS connections during shutdown.
- `mergeConfig` has no `WarmUp` default; flag default of 2 only applies when flag explicitly passed → silent 0s warmup.
- Bidirectional mode records zero latency/jitter metrics (no `recordLatency` or RTT sampling calls).
- `formatNumber` corrupts negative numbers (comma between minus sign and first digit).
- `parseHeaderIP` bracket stripping breaks bracketed IPv6 with port (`[::1]:8080` → mangled string).
- `--text-muted` CSS variable (#555566) fails WCAG AA contrast on dark backgrounds.

### Actions
- `engine.go`: replaced bare `!=` context checks with `errors.Is`; replaced `== io.EOF` with `errors.Is`; replaced `.(net.Error)` with `errors.As` (5 sites).
- Bidirectional read goroutine now records latency and samples RTT.
- Shutdown reordered: `manager.Stop()` before `wsServer.Close()` (channel closes first, goroutine exits, then WS server).
- Added `defaultWarmUp = 2` constant; `mergeConfig` now initializes `result.WarmUp = defaultWarmUp`.
- `formatNumber` handles leading minus sign before comma insertion.
- `parseHeaderIP` tries `SplitHostPort` first (handles `[::1]:8080`); bracket stripping only for bare `[::1]`.
- Bumped `--text-muted` from `#555566` to `#8888a0` for WCAG AA compliance.

## Improvement Round 14 (2026-02-06)

### Findings
- `parseFlags` returned `exitSuccess=0` for `--help`/`--version`/`--servers`; `Run()` check `exitCode != 0` fell through → nil deref panic or silent test start.
- `streamID` string read without synchronization in signal handler goroutine → data race.
- `--no-progress` flag parsed but never wired to formatter constructors → progress bars always shown.
- `StartStream` used `r.Host` for TCP/UDP addresses, ignoring `PublicHost` config → wrong addresses behind proxy.
- WebSocket `HandleStream` double-closed connection on initial write failure (explicit + defer).
- `decodeJSONBody` used bare `!= io.EOF` instead of `errors.Is`.
- `api.go` WS timeout detection used type assertion instead of `errors.As`.
- `listServers` iterated map → non-deterministic output order.
- Unknown flags (`openbyte -v`) silently started server instead of showing error.
- `srv.ListenAndServe` error check used bare `!=` for `http.ErrServerClosed`.

### Actions
- `Run()` now checks `flagConfig == nil` to handle early-exit flags (covers exitSuccess=0).
- `streamID` changed from `string` to `atomic.Value` with `.Store()`/`.Load()` for race-safe access.
- `--no-progress` wired: added `noProgress` param to both formatter constructors; `createFormatter` passes `config.NoProgress`.
- `StartStream` now checks `PublicHost` before falling back to `r.Host`, matching `GetServers` behavior.
- Removed explicit `conn.Close()` in WS initial-write error path; defer handles it.
- `decodeJSONBody` uses `stdErrors.Is(err, io.EOF)`.
- `api.go` timeout check uses `errors.As(err, &netErr)`.
- `listServers` collects aliases, sorts, then iterates in stable order.
- Removed `strings.HasPrefix(args[0], "-")` server fallback; unknown args now error with usage hint.
- `srv.ListenAndServe` uses `errors.Is(err, http.ErrServerClosed)`.
