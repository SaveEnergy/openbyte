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
- Speed test: **`speedtest-orchestrator.js`** (lifecycle + share) + thin **`openbyte.js`** init; **`speedtest-http.js`** barrels **`speedtest-http-{shared,download,upload}.js`** (shared warmup/progress via **`applyHttpMeasureTick`** in **`speedtest-http-shared.js`**); download **`download-{platform,github}.js`** + **`download.js`**; network **`network-{helpers,health}.js`** + **`network.js`** (barrel: **`network-probes.js`**, **`network-servers.js`**; **`getHealthURL`** re-exported). Any new top-level **`web/*.js`** (or HTML/CSS) must be added to **`internal/api/router_static.go`** allowlist or the server returns **404**.

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
- Deploy: **checkout first**, then `validate_env` → sync compose → remote `docker compose pull` + `up -d --force-recreate` → verify; scripts in **`scripts/deploy/`** (`validate_env`, `sync_compose`, `deploy_remote`).
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
- **Sonar**: re-check OPEN count after each Cloud analysis (query: `projects=SaveEnergy_openbyte`, `issueStatuses=OPEN`). Latest MCP pull (**2026-03-25**): **2 OPEN** — both addressed in-tree; re-run analysis to clear keys.

### Refactor analysis (2026-03-20)

- **Method**: LOC hotspot scan (`wc` on `internal/`, `pkg/`, `cmd/`, `web/`, `test/`) + import coupling read (`openbyte.js` → orchestrator + network barrel).
- **Next scan targets (when growing again)**: **`router.go`** + middleware wiring; **`speedtest_{download,upload}.go`** if HTTP speedtest grows; **`pkg/client`** (already split); optional trim **`handlers_meta.go`** / **`CompleteStream`** branch helper.
- **Web policy reminder**: new top-level **`web/*.js`** → **`internal/api/router_static.go`** allowlist + unit test (see Architecture).

### Live Queue (active)

| ID | Area | Plan | Check |
| --- | --- | --- | --- |
| — | — | **Queue drained** (2026-03-20 wave). Replenish via LOC scan + Sonar OPEN after next analysis. | — |

### Sonar OPEN (inventory — verify after next analysis)

| Issue key | Rule | Sev | Component | Action |
| --- | --- | --- | --- | --- |
| `AZ0Il3bVQcX3ClQ7_cdM` | go:S3776 | CRITICAL | was `handlers_http.go` → host helpers in **`handlers_response_host.go`** | **MCP 2026-03-20**: still **OPEN** until Cloud reanalysis; complexity target was endpoint host wiring |
| `AZ0Il3awQcX3ClQ7_cdL` | javascript:S7763 | MINOR | `web/network.js` | **MCP 2026-03-20**: still **OPEN** (stale); file uses `export { getHealthURL } from "./network-helpers.js"` — **verify CLOSED** on next analysis |

### Check hold (manual/external)

- None pending.

### Sonar snapshot

- **QG**: last Cloud check **OK** (hotspots reviewed). Re-verify **OPEN** count after next analysis.
- **OPEN (MCP check 2026-03-20)**: **2** issues — see table; JS likely stale; Go may clear after reanalysis on new file layout.
- MCP: issue search + metrics + QG; full hotspot workflow in Sonar UI.

### Recently closed (summary)

- Full ID lists live in **git history** / **CHANGELOG**.
- **2026-03-20 API handler file split**: **`handlers_response_host.go`** — `normalizeHost`, bind/proxy host wiring, **`responseHostForEndpoint`**; JSON/metrics/response helpers stay in **`handlers_http.go`**; stream snapshot DTOs + **`toStreamSnapshotResponse`** in **`handler_stream_dto.go`**, route handlers in **`handlers_stream.go`**. Checks: `go test ./internal/api/... ./test/unit/api/... -run TestOpenAPIRouteContract`.
- **2026-03-20 optional hotspot wave Done**: **`cmd/client`**: `api_stream_http.go` (REST start/cancel/complete + HTTP client helpers) + `api_websocket.go` (WS metrics loop); **`internal/api`**: `handler_stream_start.go` (stream start + validation) + slim `handlers.go`; `ratelimit_global.go` / `ratelimit_ip.go` + slim `ratelimit.go`; `speedtest_handlers.go` (Download/Upload/Ping) + slim `speedtest.go` (state + per-IP slots); **`internal/metrics`**: `collector_getmetrics.go` + slim `collector.go`. Checks: `go build ./...`, `go test ./cmd/client/... ./internal/api/... ./test/unit/api/... -run TestOpenAPIRouteContract`.
- **2026-03-20 backlog wave Done**: `20260326-refactor-01` **`cmd/loadtest`**: `config.go` + `workers.go` + thin `main.go`; `20260326-refactor-02` **web network**: `network-probes.js`, `network-servers.js`, barrel `network.js` + static allowlist; `20260322-refactor-06` **internal/api speedtest**: shared query parsers in `speedtest_query.go`; `20260322-refactor-07` **e2e**: `e2e_harness_test.go` + `e2e_stream_ws_test.go`; `20260322-refactor-08` **web**: `speedtest-orchestrator.js` + slim `openbyte.js`; `20260322-refactor-09` **mcp**: `run.go`, `tools.go`, `handlers.go` (package `mcp`); `20260323-refactor-05` **metrics**: `aggregator_merge.go`; `20260323-refactor-06` **diagnostic**: `types.go`, `interpret.go`, `ratings.go`, `assess.go`; `20260323-refactor-07` **cmd/client**: `config_yaml.go` + `config_merge.go`; `20260323-refactor-08` **internal/api**: `router_routes.go` + slim `router.go`; `20260323-refactor-09` **web**: `applyHttpMeasureTick` in `speedtest-http-shared.js` (upload + download). Checks: `go test ./cmd/client/... ./test/unit/...`, `go test ./test/e2e/... -short`, `go test ./test/unit/api/... -run TestOpenAPIRouteContract`, `npx prettier --check web/*.js`, `bunx playwright test test/e2e/ui/basic.spec.js`.
- **`20260323-refactor-10` Done**: `handler_test.go` → `handler_test_common_test.go`, `handler_auth_test.go`, `handler_crud_test.go`, `handler_validation_test.go` (`go test ./test/unit/registry/...`).
- **`20260323-refactor-01` Done**: `handlers_test.go` → `handlers_test_common_test.go` (shared stream helpers/constants) + `handlers_stream_start_test.go` (`go test ./test/unit/api/...`).
- **`20260323-refactor-02` Done**: `diagnostic_test.go` → `diagnostic_test_common_test.go`, `diagnostic_rate_test.go`, `diagnostic_interpret_test.go` (`go test ./test/unit/diagnostic/...`).
- **`20260323-refactor-03` Done**: `server_test.go` → `server_test_common_test.go`, `server_origin_test.go`, `server_broadcast_test.go`, `server_limits_test.go` (`go test ./test/unit/websocket/...`).
- **`20260323-refactor-04` Done**: `handler_test.go` → `handler_test_common_test.go`, `handler_save_test.go`, `handler_get_test.go` (`go test ./test/unit/results/...`).
- **`20260322-refactor-01` Done**: `router_test.go` → `router_test_common_test.go`, `router_middleware_stream_test.go`, `router_static_cache_ratelimit_test.go`, `router_results_api_routes_test.go`, `router_static_allowlist_test.go` (`go test ./test/unit/api/...`).
- **`20260322-refactor-02` Done**: `store_test.go` → `store_test_common_test.go`, `store_crud_test.go`, `store_retention_test.go`, `store_busy_test.go`, `store_handler_routes_test.go` (`go test ./test/unit/results/...`).
- **`20260322-refactor-03` Done**: `manager_test.go` → `manager_test_common_test.go`, `manager_lifecycle_test.go`, `manager_limits_test.go`, `manager_broadcast_metrics_test.go`, `manager_terminal_test.go` (`go test ./test/unit/stream/...`).
- **`20260322-refactor-04` Done**: `engine_dial.go`, `engine_readwrite.go`, `engine_bidirectional.go`, `engine_latency.go`; `formatter_{classify,json,plain,interactive,ndjson,helpers}.go`; slim `engine.go` (`go test ./cmd/client/...`).
- **`20260322-refactor-05` Done**: `handler_list.go`, `handler_mutations.go`; `client_http.go`, `client_loop.go`, `client_info.go`; slim `handler.go` + `client.go` (`go test ./internal/registry/... ./test/unit/registry/...`).
- **2026-03**: `20260321-refactor-01`..`03` (api tests, `pkg/client`, `cmd/client` run/http); `20260320-refactor-14`..`16`, `20260320-ci-01`..`05`, `20260320-perf-01`..`03`.
- **2026-03-19 wave**: `20260319-refactor-01`..`13` (jsonbody, websocket/api splits, web, results, stream manager, deploy scripts, config, server).
- Deferred by design: `20260226-perf-02`, `20260226-perf-04`.

### Recent decision notes

- **2026-03-20**: **`handlers_http` / stream handler seams** — host/scheme logic → **`handlers_response_host.go`**; stream JSON DTO mapping → **`handler_stream_dto.go`** (Sonar OPEN go:S3776 was still on old component key per MCP same day; re-run Cloud analysis).
- **2026-03-20**: **Backlog wave closed** — loadtest / mcp / metrics merge / diagnostic split / speedtest query / router routes / e2e harness+stream / network modules + orchestrator / client config_yaml+merge / HTTP measure dedupe (`applyHttpMeasureTick`); **AGENTS** Live Queue drained; Architecture § Frontend updated for new modules.
- **2026-03-26**: **Refactor analysis** — LOC + import coupling scan; Live Queue sharpened (evidence: file + ~LOC); new **`20260326-refactor-01`** (loadtest), **`20260326-refactor-02`** (`network.js` split + allowlist); **`20260322-refactor-09`** scoped to **MCP only** (loadtest carved out).
- **2026-03-25**: **Sonar OPEN ×2** (MCP) — go:S3776 → `handlers_http.go` helpers (`appendPortIfNonDefault`, `hostWhenBindUnspecified`, `hostForUntrustedProxy`); javascript:S7763 → `export { getHealthURL } from "./network-helpers.js"`. **`20260323-refactor-10` Done** — registry `handler_*_test.go` split. (Earlier same day: **`20260323-refactor-01`..`04`** test splits.)
- **2026-03-24**: **`20260322-refactor-01` Done** — router tests split (CORS/stream ID / static+CSP+rate-limit / results+registry+404 / allowlist+fonts+smoke); same `package api_test`, shared constants in `router_test_common_test.go`.
- **2026-03-24**: **`20260322-refactor-02` Done** — store tests split: CRUD + id charset, retention/trim, busy locks, mux handler routes (`store_handler_routes_test.go` vs `handler_test.go` direct handler tests).
- **2026-03-24**: **`20260322-refactor-03` Done** — `Manager` tests split: lifecycle, limits/concurrency, metrics channel fanout, terminal-state invariants.
- **2026-03-24**: **`20260322-refactor-04` Done** — TCP/UDP dial + connection lifecycle in `engine_dial.go`; download/upload read/write loops in `engine_readwrite.go`; bidirectional in `engine_bidirectional.go`; latency/jitter + timeout helper in `engine_latency.go`; formatters split by output mode + `formatter_classify.go`.
- **2026-03-24**: **`20260322-refactor-05` Done** — registry HTTP: core/auth + list/get vs mutations/health; outbound client: lifecycle vs POST/PUT/DELETE vs heartbeat loop/jitter vs `buildServerInfo`.
- **2026-03-23**: Complementary refactor scan → Live Queue `20260323-refactor-01`..`10` (parallel to `20260322-*`).
- **2026-03-22**: Deep LOC scan → `20260322-refactor-01`..`09`; tests first, then client/registry/e2e/web/tools.
- **2026-03-21**: `20260321-refactor-01`..`03` Done (speedtest tests, `pkg/client` files, `cmd/client` run + `http_engine_*`).
- **2026-03-20**: Refactor `14`–`16` Done (cli split, web download/network modules, `server_tcp`); CI govulncheck + Redocly pin; race/playwright/cancel-in-progress policies; perf-bench nightly; telemetry **policy** in Architecture (not implementation).
- **2026-03-20**: CI/release **`deploy`** — run **`actions/checkout` before `validate_env.sh`**; validate step invoked the script with no checkout → exit 127.
- **2026-03-20**: Local/dev UI — **`responseHostForEndpoint`** prefers **`r.Host`** when bind is unspecified (**`0.0.0.0`/`::`**) so **`api_endpoint`** matches how the user opened the app; **`router_static`** allowlist + test for **`speedtest-http-{download,shared,upload}.js`** (barrel imports).
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
