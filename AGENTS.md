## Architecture Decisions

### Core Runtime

- Single `openbyte` binary with `server` / `client` / `check` / `mcp` subcommands.
- Routing uses stdlib `net/http.ServeMux` (`METHOD /path/{param}` + `r.PathValue`).
- Web assets are embedded (`//go:embed`) with optional `WEB_ROOT` override for development.
- Stream lifecycle and counters are atomic-first on hot paths; mutexes reserved for coordination paths.

### Performance

- Fixed-bucket latency histogram (1ms buckets, 2s window) replaces sort-heavy percentile computation.
- WebSocket fanout uses single marshal per tick plus serialized writes.
- TCP/UDP code paths use buffer pooling (`sync.Pool`) and bounded goroutine patterns.
- HTTP/UI speed logic uses warm-up gating and EWMA smoothing for stable live display.
- **Advanced telemetry (policy)**: Future depth stays **server/internal first** (config-gated, logs, pprof). **Default Web UI** stays the simple speed test. **User-visible** detail requires **explicit opt-in** (env + UI or URL mode)—never default-on.

### Reliability & Concurrency

- `sync.Once` on close/stop paths for idempotent shutdown (`Manager`, registry service, stream server).
- **`internal/stream`**: `Manager` split across `manager.go`, `manager_streams.go`, `manager_cleanup.go`, `manager_broadcast.go`; `Server` across `server.go`, `server_tcp.go`, `server_udp.go` (shared `isTimeoutError` where needed).
- Shutdown order is explicit: stop producer paths first, then websocket/server teardown.
- Request/response bodies are drained on error paths to preserve HTTP/2 connection reuse.
- Stream start cleans up state if `CreateStream` succeeded but `StartStream` fails.
- Client cancel paths actively cancel server streams on context/error exits to avoid orphaned runs.

### Security & Validation

- CORS wildcard matching enforces safe dot-boundary behavior.
- CSP is strict (`script-src 'self'`), with JS moved to external files only.
- JSON API handlers enforce size limits and single-object decoding for POST/PUT payloads (`internal/jsonbody.DecodeSingleObject` shared by API + results).
- Registry auth uses constant-time compare for bearer token validation.
- Config validation includes port collision checks and trusted CIDR parsing.

### Frontend Behavior

- HTTP test mode uses `/download`, `/upload`, `/ping`; not TCP/UDP proxy mode.
- Network probe and health-check fetch paths drain non-OK and malformed JSON responses.
- Server settings UI: no custom URL mode, no synthetic "Current Server" mode, selector hidden when ≤1 reachable server.
- UI render helpers guard missing DOM nodes to avoid runtime crashes in partial layouts.
- Speed test: `speedtest*.js` + `openbyte.js` orchestration; **`speedtest-http.js`** barrels **`speedtest-http-{shared,download,upload}.js`**; download **`download-{platform,github}.js`** + **`download.js`**; network **`network-{helpers,health}.js`** + **`network.js`** (**`getHealthURL`** re-exported).

### Storage

- Results store uses SQLite (`modernc.org/sqlite`, pure Go, no CGO), WAL mode via PRAGMA.
- Share IDs are short crypto-random base62.
- Retention and max-count cleanup are enforced with periodic pruning.
- Unique constraint detection uses typed sqlite error code with fallback message match.

### Agent & API Surface

- MCP server available via `openbyte mcp` (stdio transport).
- Go SDK (`pkg/client`): `Check`, `SpeedTest`, `Diagnose`, `Healthy`; implementation split across `client.go` + `client_{check,speedtest,diagnose,health,latency,download,upload}.go` (same exported API).
- OpenAPI spec lives at `api/openapi.yaml`; CI/release lint it.
- JSON output supports schema versioning and structured error contracts.

### Build / CI / Deploy

- **Recovery**: Actions → `ci` → Run workflow on `main` if stuck; or `git fetch` via HTTPS if SSH fails.
- **`build-push` + `deploy`** on every `main` push after `checks` (path filters do not skip Docker—doc-only can still roll images).
- CI builds/pushes `edge` + `sha`; release publishes semver + `latest`.
- **`release.yml` `deploy`**: same `vars`/secrets as CI; gate on **`needs.release.result == 'success'`** (not derived job booleans).
- Deploy: sync compose → remote `docker compose pull` + `up -d --force-recreate` → verify; scripts in **`scripts/deploy/`** (`validate_env`, `sync_compose`, `deploy_remote`).
- Traefik deploy uses external `traefik` network; workflows ensure network presence.
- **Race matrix**: `ci.yml` on `main`: `go test ./... -race -short -p 1`; `nightly.yml`: full `go test -race ./...` + separate `test/e2e` (timeout budget).
- **Playwright**: `workers` = `2` on `GITHUB_ACTIONS`; optional `PLAYWRIGHT_WORKERS`; trace/reuse unchanged.
- **CI concurrency**: `cancel-in-progress` only for `pull_request`; `push`/`workflow_dispatch` queue on same `ref` (deploy not mid-aborted).
- **Nightly**: `make perf-bench` each run unless `PERF_BENCH=false`; `perf-leakcheck` still behind `LEAK_PROFILE_SMOKE`.
- **`make perf-bench`**: benches in `internal/api`, `internal/jsonbody`, plus listed unit packages; compare tips with `benchstat` (manual).

## Engineering Guardrails

- Keep behavior changes minimal and explicit; avoid orthogonal refactors in reliability passes.
- Prefer fixing root-cause over masking symptoms.
- Add regression tests for bug fixes; strengthen existing tests instead of broad rewrites.
- Keep docs aligned with actual workflow/runtime behavior after each operational change.

## Dynamic Backlog (Parallel PDCA)

### Coordination Contract

- Shared state for concurrent agents; status flow `Planned → Claimed → In Progress → Check → Done` (or `Blocked` / `Cancelled`).
- Entries need `Agent`, `Evidence`, `Check`; resolve overlaps in Decision Notes.

### Refactor backlog notes

- **Evidence**: `wc` / structure scan; before large edits use `git log --follow --stat -- <path>`. Cross-package API changes need semver + OpenAPI parity.
- **Sonar**: re-check OPEN count after each Cloud analysis (query: `projects=SaveEnergy_openbyte`, `issueStatuses=OPEN`).

### Live Queue (active)

| ID | Area | Plan | Check |
| --- | --- | --- | --- |
| `20260322-refactor-05` | registry | Split `internal/registry/handler.go` vs `client.go` (HTTP vs sync helpers). | `go test ./internal/registry/... ./test/unit/registry/...` |
| `20260322-refactor-06` | internal/api | Optional: `speedtest.go` vs `speedtest_*.go` — extract validation/deadline helpers only. | `go test ./internal/api/... ./test/unit/api/...` |
| `20260322-refactor-07` | e2e | Split `test/e2e/e2e_test.go` (~480): harness vs stream/WS vs helpers. | `go test ./test/e2e/... -short` |
| `20260322-refactor-08` | web | Reduce `ui.js` / `openbyte.js` coupling; minimal default UI change (telemetry policy). | `npx prettier --check web/*.js`; `bunx playwright test test/e2e/ui/basic.spec.js` |
| `20260322-refactor-09` | mcp / tools | When touched: `cmd/mcp/main.go` and/or `cmd/loadtest/main.go`. | `go test ./cmd/mcp/... ./test/unit/mcp/...`; `go build ./cmd/loadtest` |
| `20260323-refactor-01` | test | Split `test/unit/api/handlers_test.go` (~476); align with `handlers_*_test.go`. | `go test ./test/unit/api/...` |
| `20260323-refactor-02` | test | Split `test/unit/diagnostic/diagnostic_test.go` (~505). | `go test ./test/unit/diagnostic/...` |
| `20260323-refactor-03` | test | Split `test/unit/websocket/server_test.go` (~430): origin/broadcast/limits/ping. | `go test ./test/unit/websocket/...` |
| `20260323-refactor-04` | test | Split `test/unit/results/handler_test.go` (~454) vs store concerns. | `go test ./test/unit/results/...` |
| `20260323-refactor-05` | metrics | Split `internal/metrics` aggregator/collector vs wiring; keep atomic hot paths. | `go test ./internal/metrics/... ./test/unit/metrics/...` |
| `20260323-refactor-06` | diagnostic | Split `pkg/diagnostic/diagnostic.go`: interpretation vs thresholds vs `Interpret`. | `go test ./pkg/diagnostic/...` |
| `20260323-refactor-07` | cmd/client | Split `config.go` vs `api.go` (flags vs REST); orthogonal to `20260322-refactor-04`. | `go test ./cmd/client/...` |
| `20260323-refactor-08` | internal/api | Optional: `router.go` / `handlers.go` seams; keep `TestOpenAPIRouteContract` green. | `go test ./internal/api/...`; `go test ./test/unit/api/... -run TestOpenAPIRouteContract` |
| `20260323-refactor-09` | web | Further dedupe `speedtest-http-upload.js` / `download.js` vs `speedtest-http-shared.js`. | `npx prettier --check web/*.js`; `bunx playwright test test/e2e/ui/basic.spec.js` |
| `20260323-refactor-10` | test / tools | Split `test/unit/registry/handler_test.go` vs `service_test.go`; optional `cmd/check/main.go`. | `go test ./test/unit/registry/...`; `go test ./cmd/check/...` |

### Check hold (manual/external)

- None pending.

### Sonar snapshot

- **QG**: last Cloud check **OK** (hotspots reviewed). Re-verify **OPEN** count after next analysis.
- MCP: issue search + metrics + QG; full hotspot workflow in Sonar UI.

### Recently closed (summary)

- Full ID lists live in **git history** / **CHANGELOG**.
- **`20260322-refactor-01` Done**: `router_test.go` → `router_test_common_test.go`, `router_middleware_stream_test.go`, `router_static_cache_ratelimit_test.go`, `router_results_api_routes_test.go`, `router_static_allowlist_test.go` (`go test ./test/unit/api/...`).
- **`20260322-refactor-02` Done**: `store_test.go` → `store_test_common_test.go`, `store_crud_test.go`, `store_retention_test.go`, `store_busy_test.go`, `store_handler_routes_test.go` (`go test ./test/unit/results/...`).
- **`20260322-refactor-03` Done**: `manager_test.go` → `manager_test_common_test.go`, `manager_lifecycle_test.go`, `manager_limits_test.go`, `manager_broadcast_metrics_test.go`, `manager_terminal_test.go` (`go test ./test/unit/stream/...`).
- **2026-03**: `20260321-refactor-01`..`03` (api tests, `pkg/client`, `cmd/client` run/http); `20260320-refactor-14`..`16`, `20260320-ci-01`..`05`, `20260320-perf-01`..`03`.
- **2026-03-19 wave**: `20260319-refactor-01`..`13` (jsonbody, websocket/api splits, web, results, stream manager, deploy scripts, config, server).
- Deferred by design: `20260226-perf-02`, `20260226-perf-04`.

### Recent decision notes

- **2026-03-24**: **`20260322-refactor-01` Done** — router tests split (CORS/stream ID / static+CSP+rate-limit / results+registry+404 / allowlist+fonts+smoke); same `package api_test`, shared constants in `router_test_common_test.go`.
- **2026-03-24**: **`20260322-refactor-02` Done** — store tests split: CRUD + id charset, retention/trim, busy locks, mux handler routes (`store_handler_routes_test.go` vs `handler_test.go` direct handler tests).
- **2026-03-24**: **`20260322-refactor-03` Done** — `Manager` tests split: lifecycle, limits/concurrency, metrics channel fanout, terminal-state invariants.
- **2026-03-24**: **`20260322-refactor-04` Done** — TCP/UDP dial + connection lifecycle in `engine_dial.go`; download/upload read/write loops in `engine_readwrite.go`; bidirectional in `engine_bidirectional.go`; latency/jitter + timeout helper in `engine_latency.go`; formatters split by output mode + `formatter_classify.go`.
- **2026-03-23**: Complementary refactor scan → Live Queue `20260323-refactor-01`..`10` (parallel to `20260322-*`).
- **2026-03-22**: Deep LOC scan → `20260322-refactor-01`..`09`; tests first, then client/registry/e2e/web/tools.
- **2026-03-21**: `20260321-refactor-01`..`03` Done (speedtest tests, `pkg/client` files, `cmd/client` run + `http_engine_*`).
- **2026-03-20**: Refactor `14`–`16` Done (cli split, web download/network modules, `server_tcp`); CI govulncheck + Redocly pin; race/playwright/cancel-in-progress policies; perf-bench nightly; telemetry **policy** in Architecture (not implementation).
- **2026-03-19**: Large refactor wave `01`–`13` + v0.8.0; Go 1.26.x baseline; Sonar OPEN query parity.
- **2026-03-07 / 03-01 / 02-26**: Earlier closure waves (web resilience, Sonar targets, security/perf items)—details in CHANGELOG and old commits.

### Verification baseline

- `go test ./cmd/check ./cmd/mcp ./cmd/server ./cmd/client`
- `go test ./test/unit/api ./test/unit/client ./test/unit/mcp ./test/unit/results ./test/unit/websocket`
- `go test ./internal/results`

### Test layout

- Prefer `test/`; legacy white-box tests may stay under `cmd/` / `internal/` for package-private access.

## Open / Deferred

- Rich telemetry UI beyond Architecture § Performance policy.
- Public hosted test fleet (infra/cost).
- Additional SDKs from OpenAPI (TypeScript/Python).
- Packaging polish (Homebrew/apt).
