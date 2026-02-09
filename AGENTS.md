## Architecture Decisions

### Performance
- Fixed-bucket latency histogram (1ms buckets, 2s window) avoids O(n log n) percentile sorts.
- WebSocket fanout: single marshal per tick + serialized writes (not per-client marshal).
- Buffer pooling (`sync.Pool`) for TCP/UDP receive buffers; pre-generated 1MB random data block for downloads.
- Atomic counters on hot paths (bytes, packets, active count); mutexes only for cold paths.
- `LatencyHistogram` uses `sync.RWMutex` — writers exclusive, readers (`CopyTo`) concurrent.
- Manager `ActiveCount()` is O(1) via `atomic.LoadInt64` (not map scan).

### Concurrency & Shutdown
- `sync.Once` on `Manager.Stop()` and `RegistryService.Stop()` for idempotent shutdown.
- Broadcast goroutine tracked by `sync.WaitGroup`; shutdown waits for exit before closing WebSocket server.
- Shutdown order: `manager.Stop()` → `wsServer.Close()` (channel closes first, then WS).
- TCP connection limit (`activeTCPConns` atomic + `maxTCPConns`) with hard duration cap per connection.
- UDP sender concurrency limit with double-checked locking after write lock (TOCTOU fix).
- UDP read parallelism: `min(GOMAXPROCS, 4)` reader goroutines on single `UDPConn`; `udpClients` struct owns map+mutex; ticker-based cleanup (not inline).
- Client `streamID` uses `atomic.Value` for race-safe signal handler access.

### Rate Limiting
- Token bucket with fractional remainder preservation (avoids `int()` truncation losing tokens).
- Per-IP: `rightmostUntrustedIP` walks XFF right-to-left, skipping trusted proxy CIDRs (prevents spoofing).
- Upload handler has duration cap + context cancellation (prevents slow-drip slot exhaustion).
- Download/upload concurrency uses atomic increment-then-check (eliminates TOCTOU).

### Metrics
- Jitter: RFC 3550 mean consecutive difference (both server-side and client RTTCollector).
- Percentile: `math.Ceil(count * ratio)` — no off-by-one.
- Packet loss clamped ≥ 0 (atomic timing can cause `recv > sent`).
- RTTCollector uses ring buffer (fixed backing array, head/count indices) — no reslice leak.

### Security
- CORS wildcard: dot-boundary enforcement (`*.example.com` won't match `evilexample.com`).
- CSP: `script-src 'self'`; all JS in external files (no inline scripts).
- CSP `connect-src` narrowed from wildcard to explicit `'self' https: http: ws: wss:` (keeps API/WebSocket/server-probe flexibility without blanket allow-all).
- Playwright regression: `skill.html` asserts no inline scripts (`<script src=...>` only) to prevent CSP regressions.
- Static HTML responses (`/`, `*.html`, including `/results/{id}`) send `Cache-Control: no-store` to avoid stale page/script mismatches after deploy.
- WebSocket `SetReadLimit(4096)` — server only reads for disconnect detection.
- JSON body size limits on all POST endpoints.
- Stream IDs: UUID format enforced at routing layer.
- Registry API key compared with `subtle.ConstantTimeCompare`.
- Port collision validation (HTTP vs TCP/UDP) in config.

### HTTP Streaming (Web UI)
- Web UI uses HTTP endpoints (`/download`, `/upload`, `/ping`), not TCP/UDP proxy mode.
- EWMA-smoothed live speed display (alpha=0.3); final result uses raw `totalBytes / elapsed`.
- Dynamic warm-up: rolling throughput windows, settles when last 3 vary < 15%.
- 24 latency pings, 2 warm-up discarded, IQR outlier filter.
- Loaded latency probe (500ms interval) during download/upload for bufferbloat grading.
- Multi-step chunk fallback on download errors (1MB → 256KB → 64KB).
- Cancel+restart: captured `signal` threaded through all phases; `finally` checks signal ownership before cleanup.
- All fetch responses drained (success and error paths) for HTTP/2 stream reuse.

### Frontend Signal Threading
- `startTest()` captures local `signal = state.abortController.signal`.
- All phase functions (`measureLatency`, `runTest`, `runDownloadTest`, `runUploadTest`, `startLoadedLatencyProbe`) accept and use this signal.
- `finally` block only cleans up if `state.abortController?.signal === signal` (prevents old test clobbering new).
- `fetchWithTimeout` chains caller's abort signal with timeout controller; removes listener on cleanup (prevents accumulation).
- `download.js` uses DOM manipulation (no `innerHTML`) for dynamic content.
- Download page release rendering now guards invalid asset URLs/sizes and invalid publish dates before UI updates.
- Download page release fetch drains non-OK response bodies before throwing (connection reuse).

### Storage
- SQLite via `modernc.org/sqlite` (pure Go, no CGO) for cross-compilation.
- WAL mode via explicit PRAGMA (not query string — driver ignores it).
- 8-char base62 IDs via `crypto/rand`; single `rand.Read(8)` call per ID (not 8 individual syscalls).
- 90-day retention + configurable max count with hourly cleanup.
- Unique-constraint detection checks typed sqlite error code (`SQLITE_CONSTRAINT_UNIQUE`) with message fallback, not message match only.

### Agent Integration
- MCP server (`openbyte mcp`) over stdio transport; 3 tools: `connectivity_check`, `speed_test`, `diagnose`. Uses `github.com/mark3labs/mcp-go`.
- Go SDK in `pkg/client/`: `New()`, `Check()`, `SpeedTest()`, `Diagnose()`, `Healthy()` — wraps HTTP engine; no shell needed.
- Quick check (`openbyte check`): 3-5s latency + burst download + burst upload; returns grade A-F + interpretation. Exit code 0 = healthy, 1 = degraded.
- Diagnostic interpretation in `pkg/diagnostic/`: grades (A-F), ratings (latency/speed/stability), suitability list, concerns. Shared by CLI, MCP, SDK.
- Structured JSON errors: `--json` emits `{"error":true,"code":"...","message":"..."}` to stdout (not unstructured stderr). Error codes: `connection_refused`, `timeout`, `rate_limited`, `server_unavailable`, `invalid_config`, `cancelled`, `network_error`.
- Schema versioning: `schema_version: "1.0"` on all JSON output (results + errors). Semver independent of binary version.
- NDJSON streaming: `--ndjson` flag emits `{"type":"progress",...}` per tick + `{"type":"result","data":{...}}` final line.
- OpenAPI 3.1 spec at `api/openapi.yaml` covering all `/api/v1/*` + `/health` endpoints.
- Install script: `scripts/install.sh` (curl|sh, detects OS/arch, downloads from GitHub Releases).
- Installer creates missing `INSTALL_DIR` (with `sudo` when needed) and verifies extracted binary exists before copy.
- Installer rejects empty `INSTALL_DIR` before path operations to avoid ambiguous install targets.
- `go install github.com/saveenergy/openbyte/cmd/openbyte@latest` works (entry point at `cmd/openbyte/`).

### Build & Deploy
- Single `openbyte` binary with `server`/`client`/`check`/`mcp` subcommands.
- `openbyte server` now accepts deploy-oriented CLI flags (ports, server identity, limits, registry/proxy options); when explicitly set, flags override env-derived config.
- Web assets embedded via `//go:embed` (HTML, CSS, JS, fonts).
- Self-hosted fonts (DM Sans, JetBrains Mono) — no external CDN dependencies.
- Deploy scripts pin image tags explicitly (`IMAGE_TAG=$GITHUB_SHA` on CI main, `IMAGE_TAG=$SEMVER` on release tags) so pull/deploy targets deterministic image.
- Deploy uses `docker compose up -d --force-recreate` after pull to ensure running container is replaced even when tag string is unchanged (`edge`/`latest` patterns).
- Deploy scripts verify running container image ID matches expected GHCR image ID after recreate; owner normalized to lowercase for GHCR path compatibility.
- Deploy jobs sync `docker-compose.ghcr.yaml` + `docker-compose.ghcr.traefik.yaml` from pipeline workspace to `${REMOTE_DIR}/docker` before remote compose commands, preventing server-side compose drift.
- Deploy compose sync step verifies remote file SHA-256 checksums match workspace files before executing `docker compose`.
- Deploy sync hardening: fail-fast `ssh-keyscan`, explicit remote compose file existence checks post-`scp`, and `always()` cleanup of temporary SSH key material on runner.
- Deploy script propagates bash-script exit status (`exit $?`) and enforces `GHCR_OWNER` override during compose pull/up to avoid owner drift across server `.env` and workflow context.
- Deploy pull step is explicit fail-fast (`docker compose pull || exit 1`) before recreate.
- Deploy sync validates `REMOTE_DIR` is non-empty before remote mkdir/scp/deploy paths are used.
- Deploy sync uses `timeout 15 ssh-keyscan ...` and verifies non-empty host-key output before updating `known_hosts`.
- Deploy step verifies `${REMOTE_DIR}/.env` exists before compose calls; fallback shell path now uses `set -euo pipefail`.
- GHCR Traefik compose uses external `traefik` network; deploy ensures network exists (`docker network inspect ... || docker network create`) before compose to avoid label mismatch failures on pre-existing host networks.
- `WEB_ROOT` env overrides embedded FS for development.
- Playwright NO_COLOR handling: CI step uses `env -u NO_COLOR ...`; npm scripts use `scripts/run-playwright.mjs` (cross-platform, unsets `NO_COLOR` before spawning `bunx playwright`).
- `ReadTimeout: 0` (disabled); `ReadHeaderTimeout: 15s` (slowloris protection without killing uploads).
- HTTP concurrency auto-scales from `CAPACITY_GBPS` via `MaxConcurrentHTTP()`.
- Client env vars (`OBYTE_*`) removed; flags only. `NO_COLOR` standard convention retained.
- `ServerInfo` shared via `pkg/types/server.go` (deduplicated from `api` + `registry`).
- `buildServerInfo` derives scheme from `TrustProxyHeaders` (not hardcoded `http://`).
- Client context timeout adds test duration to base timeout (lifecycle ≠ test phase).
- TCP/UDP warm-up: data flows during first `WarmUp` seconds but `addBytes` gates recording; one-time CAS resets counters/`measureStart` at transition. Same pattern as HTTP engine `graceBytes`/`graceDone`. Test context runs for `WarmUp + Duration`.
- Routing: stdlib `net/http.ServeMux` (Go 1.22+); `"METHOD /path/{param}"` patterns, `r.PathValue("param")`. `RegistryRegistrar` interface for external route registration before middleware wrapping.

### Error Handling Patterns
- All `io.EOF` / `net.Error` checks use `errors.Is` / `errors.As` (not bare `==` / type assertion).
- `http.ErrServerClosed` uses `errors.Is` everywhere.
- All API errors return JSON `{"error":"..."}` (no plaintext `http.Error`).
- Registry handlers validate `Content-Type: application/json`.
- API stream start/metrics/complete handlers now also enforce JSON Content-Type (while allowing omitted header for compatibility).
- Cancel-stream API now drains/closes request bodies before cancellation to keep HTTP connection reuse safe.
- `json.Encode` errors logged (not silently dropped).
- Response bodies drained before close for HTTP connection reuse.
- Tests hardened: JSON decode/marshal errors are asserted in e2e/unit paths (no ignored decode/marshal results).
- E2E map field extraction avoids direct type-assert panics (`value.(string)`): checks `ok` + non-empty first for clearer failures.
- Registry handler tests validate `count` field presence/type before numeric comparison (avoid direct `resp["count"].(float64)` panics).
- Integration tests validate response field types (not just non-nil), e.g. `stream_id`/`websocket_url` must be non-empty strings.
- Results API GET now uses `Cache-Control: no-store` (matches HTML no-store policy) and unit test coverage checks this header.
- Results save handler enforces single JSON object body (rejects concatenated JSON payloads) and drains on parse violations.
- `loadServers` drains non-OK `/servers` responses before error path.
- Results page parser trims trailing slashes when extracting result IDs and logs fetch failures; render path now guards non-finite speed values.
- Playwright cancel/restart test waits for UI states instead of fixed sleep to reduce CI flakiness.
- Frontend server-health indicators (`serverDot`/`serverText`) now tolerate missing DOM nodes to avoid runtime null dereferences.
- Toast error helpers now guard missing toast elements (`errorToast`, `errorMessage`) to keep embedded/minimal layouts safe.
- Copy button handlers on skill/download pages guard missing `.dl-code-block`/`code` nodes before dereferencing.
- Frontend bind/init flow now exits safely when core controls are missing (`startBtn`, `restartBtn`, `duration`, `streams`) and logs a warning instead of throwing.
- Web results page now validates required view containers upfront and drains non-OK fetch responses before surfacing HTTP errors.
- HTTP client engine now fails fast if upload payload randomization fails (`crypto/rand.Read`), returning constructor error to caller.
- JSON request decode helpers now drain unread request bodies on decode/single-object validation errors (`internal/api`, `internal/results`, `internal/registry`) to preserve connection reuse safety.
- Registry register/update handlers now enforce single-object JSON bodies (reject concatenated payloads).
- Client stream-start decode path drains response body on JSON decode errors before returning.
- HTTP client engine now drains/closes non-nil responses on `Do` error paths in both download and upload loops.
- CLI fastest-server health checks now drain/close non-nil responses even on `Get` error paths.
- Proxy-mode CLI now cancels server stream if websocket metrics loop exits with error; client-mode cancel path also cancels server stream on context cancellation.
- Settings load/save paths guard missing duration/streams controls, log parse failures, and server discovery now validates `servers` as an array before assignment.
- CI workflow cleanup: removed unused `Force build override` step from `changes` job.
- Deploy jobs now include an explicit "Validate deploy configuration" step to fail fast when required vars/secrets (`SSH_*`, `REMOTE_DIR`, `GHCR_*`) are missing.
- Client `startStream` now surfaces `io.ReadAll` failures on non-201 responses instead of silently ignoring body-read errors.
- Web download test now drains non-OK HTTP responses before fallback handling to preserve browser connection reuse behavior.
- Address-family network probes (`v4.`/`v6.` ping) now drain non-OK responses before rejection.
- Main `/ping` network probe now checks `res.ok` and drains non-OK responses before rejection.
- `loadServers` now treats malformed JSON as a hard failure and clears server list fallback state.
- E2E static-file checks drain non-OK response bodies before close.
- Download/results pages add broader DOM null-guards to avoid runtime crashes in partial/minimal layouts.
- Release workflow `image_pushed` output now emits boolean (`true` on successful push step), and deploy gate checks `== 'true'` for clearer control flow.
- Client cancel/complete stream helpers now defensively drain/close non-nil HTTP responses on `Do` errors before returning.
- UI speed/progress/state render helpers now short-circuit when required DOM nodes are absent; share state only enables when result ID is valid.
- Client `startStream` now enforces a defensive minimum HTTP timeout (60s) when `config.Timeout <= 0` to avoid indefinite waits.
- Multi-stream collector selection now computes round-robin index using bounded uint32 modulo to avoid signed overflow edge cases.
- Results cleanup now handles and logs `RowsAffected()` errors instead of ignoring them.
- Deploy job gates now require `REMOTE_DIR` and `GHCR_USERNAME` vars (in addition to `SSH_HOST`) before attempting sync/deploy.
- Registry client `drainAndClose` now no-ops on nil response/body to prevent defensive-path panics.
- Results page rendering now ignores invalid `created_at` timestamps and enforces string-safe rendering for `bufferbloat_grade` and `server_name`.
- Speedtest upload handler now drains remaining request body before returning on success/error paths to improve connection reuse behavior.
- `computeBufferbloatGrade` now rejects non-finite latency values early; results page numeric guards now consistently use `Number.isFinite`.
- UI connected-state e2e matcher includes `Custom`/`Unverified` states for custom-server scenarios.
- Deploy sync checksum verification now supports both `sha256sum` and `shasum -a 256` on remote hosts, improving Linux/macOS compatibility.
- Deploy job gates now also require `SSH_USER` var before running deploy in both CI and release workflows.
- Install script now requests GitHub API JSON explicitly and prefers `jq` for `tag_name` parsing with a POSIX fallback parser.

### Dead Code Removed
- QUIC server/client support (2026-02-02).
- `ErrConnectionFailed`, `IsContextError`, `ErrCodeTimeout`, `ErrCodeCancelled` from `pkg/errors`.
- `MeasureRTT` → `measureRTT` (unexported).
- `Collector.GetSnapshot`, `MultiStreamAggregator.GetCollector`.
- `sendPool` / `getSendBuffer` from stream server.
- `activeStreams` map / `StreamSession` struct from stream server.
- `isPublicIP`, `firstClientIP` from clientip.
- `saveConfigFile` from client config.
- `stringsTrimSuffix` wrapper (inlined `strings.TrimRight`).
- Dead CSS (`.counting`, `countUp` keyframe, overridden `border: none`).
- Deprecated `version: '3.8'` from compose files.
- TCP/UDP `runWarmUp` no-op removed; warm-up integrated into data phase via `addBytes` gating.
- `math/big` import removed from `results/store.go` (replaced by `rand.Read`).
- `gorilla/mux` dependency removed; migrated to Go 1.22+ `net/http.ServeMux` (`{name}` path params + `METHOD /path` patterns).
- `RateLimitMiddleware` (gorilla middleware adapter) replaced by `applyRateLimit` (direct `HandlerFunc` wrapper).

## Open / Deferred

- Public test servers (infrastructure cost; defer until adoption justifies).
- Python/TypeScript SDKs (auto-generate from OpenAPI spec).
- WebSocket MCP transport (stdio is standard for local tools).
- Homebrew formula / apt repo (distribution polish).

## Changelog Summary

### v0.5.0 (2026-02-05)
- Agent integration layer: MCP server (`openbyte mcp`), Go SDK (`pkg/client/`), quick check (`openbyte check`), diagnostic interpretation (`pkg/diagnostic/`).
- Structured JSON errors with machine-readable error codes when `--json` active.
- Schema versioning (`schema_version: "1.0"`) on all JSON output.
- NDJSON streaming formatter (`--ndjson`) for real-time progress.
- OpenAPI 3.1 spec (`api/openapi.yaml`).
- Install script (`scripts/install.sh`) for one-liner installation.
- Interpretation layer in all results: grade, ratings, suitability, concerns.
- New dependency: `github.com/mark3labs/mcp-go` (MCP protocol).
- `cmd/check` and `cmd/mcp` deduplicated: now use `pkg/client` SDK instead of reimplementing HTTP helpers (net -348 lines).
- `bytes.NewReader(payload)` replaces `strings.NewReader(string(payload))` — avoids 1MB unnecessary copy in upload bursts.
- `json.Encode`/`MarshalIndent` errors now handled in NDJSON formatter, check command, and MCP handlers (no silent failures).

### v0.4.2 (2026-02-08)
- Migrated `gorilla/mux` → Go stdlib `net/http.ServeMux`; `"METHOD /path/{param}"` patterns + `r.PathValue()`. Removed `RateLimitMiddleware`. `RegistryRegistrar` interface for external route registration.
- UDP multi-reader: N goroutines (`min(GOMAXPROCS, 4)`, floor 2) call `ReadFromUDP` concurrently on same `UDPConn`. Client state extracted to `udpClients` struct; cleanup on `time.Ticker` instead of inline in hot path. No more `SetReadDeadline` polling — readers block until packet or connection close.
- `ServerInfo` dedup to `pkg/types`; `generateID` batched entropy; `buildServerInfo` TLS-aware scheme.
- `fetchWithTimeout` listener leak fix; `selectFastestServer` body drain; `sentStatus` cleanup for clientless streams.
- `navigator.platform` → `userAgentData`; `innerHTML` → DOM manipulation.
- TCP/UDP warm-up integrated into data phase via `addBytes` gating (no-op sleep removed).
- Client context timeout separated from test duration; HTTP engine overhead/grace wired from config.
- Makefile `perf-smoke` uses built binary; Playwright CI caching.
- Flaky concurrency tests replaced with signal-based sync; Registry API documented in `API.md`.
- Test coverage expansion: registry service + handler (15 tests), stream manager (16 tests), latency histogram (12 tests), RTT collector (10 tests), network helpers (7 tests), API handlers expanded (19 new tests covering GetVersion, GetServers, ReportMetrics, CompleteStream, CancelStream, GetStreamResults, validation edge cases). Agent-friendly layer tests: diagnostic interpretation (42 tests: latency/speed/stability rating boundaries, grade computation A-F, suitability workloads, concerns, summary formatting, nil-safety), formatter (16 tests: JSON structured errors with all error codes, schema versioning, NDJSON progress/metrics/complete/error streaming, multiline output), SDK (11 tests: health check OK/unreachable/unhealthy, speed test download/upload/default/invalid/clamped/unreachable, API key option, Check interpretation), MCP integration (6 tests: connectivity check/speed test/diagnose via SDK, error readability, grade table-driven, JSON schema validation).
- HTTP download error path now drains response body before close (HTTP/2 connection reuse).
- Negative `PacketSize` defaults to 1400 instead of passing validation.
- Registry `UpdateServer` rejects body ID conflicting with URL path param.
- CIDR parse errors logged as warnings (no longer silently skipped).
- `copy` builtin shadowing fixed in `registry/service.go`.
- Last `innerHTML` usage replaced with DOM manipulation (`serverSelect` clearing).
- UDP packet size default 1500 → 1400: avoids IP fragmentation on 1500-byte MTU links (1400 payload + 28 IPv4/UDP headers = 1428 < 1500). Also safe for IPv6 (1400 + 48 = 1448) and common tunnel encapsulations (PPPoE, VPN).
- `InteractiveFormatter.FormatComplete` de-bloated: extracted color helper, eliminated 30-line color/no-color duplication.
- Results handler: added `math.IsNaN`/`math.IsInf` defense-in-depth validation on all float64 fields.
- `completeStream` now logs errors to stderr instead of silently swallowing.
- `download.js`: modernized `var` → `const`/`let` throughout.
- `diagnostic.suitability()`/`concerns()` return `[]string{}` (not nil) — prevents JSON `null` in agent output.

### v0.4.1 (2026-02-07)
18 improvement rounds covering: error handling (`errors.Is`/`errors.As` throughout), cancel+restart race fixes (signal threading), rate limiter token refill fix (40% throughput recovery), CSP headers + inline script extraction, `ReadHeaderTimeout` replacing `ReadTimeout`, self-hosted fonts, client env var removal, latency histogram mutex upgrade, dead code cleanup, documentation overhaul.

### v0.4.0 (2026-02-05)
SQLite result storage + share button, HTTP streaming CLI support, embedded web assets, download page redesign, IPv4/IPv6 detection, EWMA speed smoothing, dynamic warm-up, loaded latency/bufferbloat grading, capacity-derived concurrency limits, security hardening (CORS wildcard fix, XFF rightmost-untrusted, TCP/UDP connection limits, WebSocket read limit, upload duration cap).

### Pre-v0.4.0 (2026-01-16 – 2026-02-01)
Performance optimizations (histogram, buffer pooling, atomic counters), CI/CD pipeline (GitHub Actions, GHCR, release workflow, dependabot), unified binary refactor, QUIC removal, test relocation to `test/unit/`, Playwright E2E setup, reliability fixes.
