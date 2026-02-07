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

### Storage
- SQLite via `modernc.org/sqlite` (pure Go, no CGO) for cross-compilation.
- WAL mode via explicit PRAGMA (not query string — driver ignores it).
- 8-char base62 IDs via `crypto/rand`; single `rand.Read(8)` call per ID (not 8 individual syscalls).
- 90-day retention + configurable max count with hourly cleanup.

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
- `go install github.com/saveenergy/openbyte/cmd/openbyte@latest` works (entry point at `cmd/openbyte/`).

### Build & Deploy
- Single `openbyte` binary with `server`/`client`/`check`/`mcp` subcommands.
- Web assets embedded via `//go:embed` (HTML, CSS, JS, fonts).
- Self-hosted fonts (DM Sans, JetBrains Mono) — no external CDN dependencies.
- `WEB_ROOT` env overrides embedded FS for development.
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
- `json.Encode` errors logged (not silently dropped).
- Response bodies drained before close for HTTP connection reuse.

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
