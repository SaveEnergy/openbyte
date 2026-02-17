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

### Reliability & Concurrency

- `sync.Once` on close/stop paths for idempotent shutdown (`Manager`, registry service, stream server).
- Shutdown order is explicit: stop producer paths first, then websocket/server teardown.
- Request/response bodies are drained on error paths to preserve HTTP/2 connection reuse.
- Stream start cleans up state if `CreateStream` succeeded but `StartStream` fails.
- Client cancel paths actively cancel server streams on context/error exits to avoid orphaned runs.

### Security & Validation

- CORS wildcard matching enforces safe dot-boundary behavior.
- CSP is strict (`script-src 'self'`), with JS moved to external files only.
- JSON API handlers enforce size limits and single-object decoding for POST/PUT payloads.
- Registry auth uses constant-time compare for bearer token validation.
- Config validation includes port collision checks and trusted CIDR parsing.

### Frontend Behavior

- HTTP test mode uses `/download`, `/upload`, `/ping`; not TCP/UDP proxy mode.
- Network probe and health-check fetch paths drain non-OK and malformed JSON responses.
- Server settings UI is simplified:
  - no custom URL mode,
  - no synthetic "Current Server" mode,
  - selector hidden when <=1 reachable server.
- UI render helpers guard missing DOM nodes to avoid runtime crashes in partial layouts.

### Storage

- Results store uses SQLite (`modernc.org/sqlite`, pure Go, no CGO), WAL mode via PRAGMA.
- Share IDs are short crypto-random base62.
- Retention and max-count cleanup are enforced with periodic pruning.
- Unique constraint detection uses typed sqlite error code with fallback message match.

### Agent & API Surface

- MCP server available via `openbyte mcp` (stdio transport).
- Go SDK (`pkg/client`) exposes `Check`, `SpeedTest`, `Diagnose`, `Healthy`.
- OpenAPI spec lives at `api/openapi.yaml`; CI/release lint it.
- JSON output supports schema versioning and structured error contracts.

### Build / CI / Deploy

- CI main builds/pushes `edge` + `sha`; release pipeline publishes semver + `latest`.
- Deploy path syncs compose files before remote execution to prevent server-side drift.
- Deploy runs `docker compose pull` + `up -d --force-recreate`, then verifies expected image/container state.
- Traefik deploy uses external `traefik` network; workflows ensure network presence.
- Workflow gates require required deploy vars/secrets and fail fast on missing config.

## Engineering Guardrails

- Keep behavior changes minimal and explicit; avoid orthogonal refactors in reliability passes.
- Prefer fixing root-cause over masking symptoms.
- Add regression tests for bug fixes; strengthen existing tests instead of broad rewrites.
- Keep docs aligned with actual workflow/runtime behavior after each operational change.

## Dynamic Backlog (Parallel PDCA)

### Coordination Contract

- Treat this section as shared state for concurrent agents.
- Use monotonic status flow only: `Planned -> Claimed -> In Progress -> Check -> Done` (or `Blocked` / `Cancelled`).
- Keep entries attributable (`Agent`, `UTC`, `Evidence`, `Check`).
- If overlaps happen, resolve explicitly in `Decision Notes` (no silent overwrite).

### Work Item Schema

- `ID`: `YYYYMMDD-<area>-<nn>`
- `Area`: `api`, `client`, `web`, `ci`, `docs`, ...
- `Agent`: owner tag (`A0`, `A1`, ...)
- `Status`: allowed state
- `Plan`: one-line intent
- `Evidence`: concrete proof
- `Check`: exact verification command

### Live Queue (active only)

| ID       | Area | Agent | Status | Plan                                                         | Evidence | Check |
| -------- | ---- | ----- | ------ | ------------------------------------------------------------ | -------- | ----- |
| 20260217-web-02 | web | A0 | Check | Burn down highest-open frontend static-analysis backlog in `web/results.js`, `web/app.js`, `web/download.js`. | Batch-1 implemented: `web/results.js` modernized (`var` removal, optional chaining, `window` -> `globalThis`, explicit catch handling); `web/download.js` updated for optional chaining/deprecation-safe patterns/removeChild modernizations; `web/app.js` targeted S6582/S7773/S7764 fixes (`Number.parseInt`, optional chaining, constant cleanup). | `npx prettier --check web/*.js && bunx playwright test` |
| 20260217-go-02 | api | A0 | Check | Continue production Go hotspot reduction (`go:S3776`) in server/api/websocket/stream paths. | Batch-1: extracted download parsing/source/streaming helpers in `internal/api/speedtest.go`; split origin matching logic in `internal/websocket/server.go`. Batch-2: reduced orchestration complexity in `cmd/server/main.go` via lifecycle helpers (`startHTTPServer`, `waitForShutdown`, `shutdownHTTPServer`, `stopServerDependencies`) and collapsed repeated timeout/read-error branches in `internal/stream/server.go` (`isTimeoutError`, `isRetryableConnReadError`). | `go test -short ./cmd/server ./cmd/client ./internal/api ./internal/stream ./internal/websocket` |
| 20260217-test-02 | test | A0 | Check | Bulk-close low-risk test-only smells (`go:S100`, `go:S1192`, `godre:S8193`) via focused cleanup sweeps. | Batch-1: `test/unit/client/sdk_test.go` centralized repeated literals (endpoint paths, content-type, common status payload, direction values, unreachable URL). Batch-2: `test/unit/diagnostic/diagnostic_test.go` centralized repeated ratings/suitability/concern literals and replaced repeated assertions with constants. Strict OPEN recheck currently still reports `422` total (`go:S100=102`, `go:S1192=102`, `godre:S8193=50`), so next step is waiting for refreshed server-side analysis before assessing net reduction. | `go test -short ./test/unit/... ./test/e2e/...` |

### Sonar Snapshot (2026-02-17)

- Strict OPEN filter parity maintained with Cloud:
  - Query: `projects=[SaveEnergy_openbyte]`, `issueStatuses=[OPEN]`, `ps=500`
  - Total OPEN: `422`
  - Current top tracked rules: `go:S3776=36`, `go:S100=102`, `go:S1192=102`, `godre:S8193=50`

### Recently Closed IDs

- `20260205-api-05`, `20260205-client-05`, `20260205-client-06`, `20260205-ci-02`, `20260205-ci-03`, `20260205-docs-02`
- `20260215-ci-07`, `20260215-docker-03`, `20260215-check-01`, `20260215-check-02`, `20260215-check-03`, `20260215-check-04`
- `20260215-client-09`, `20260215-sdk-01`, `20260215-config-02`, `20260215-registry-03`, `20260215-config-03`, `20260215-config-04`
- `20260215-results-03`, `20260215-results-04`, `20260215-results-05`, `20260215-openbyte-02`, `20260215-mcp-01`, `20260215-api-06`
- `20260215-api-07`, `20260215-loadtest-01`, `20260215-loadtest-02`, `20260215-metrics-01`, `20260215-install-01`, `20260215-client-10`
- `20260215-mcp-02`, `20260215-diagnostic-01`
- `20260216-ci-08`, `20260216-go-01`, `20260216-test-01`, `20260216-web-01`, `20260216-scripts-01`, `20260216-cleanup-01`
- `20260217-ci-09`

### Recent Decision Notes

- Adopted Go 1.26 baseline (`go.mod` + CI/nightly/release `setup-go` pin to `1.26.x`).
- Ran `go fix ./...` and kept behavior-neutral modernizers (e.g., `any`, built-in `min/max`, small stdlib rewrites); skipped behavior-changing `omitempty -> omitzero`.
- Added optional leak-debug path via `make perf-leakcheck` using `GOEXPERIMENT=goroutineleakprofile` + pprof `goroutineleak` endpoint capture.
- Used package-internal white-box tests for rollback/mapping branches hard to trigger from black-box HTTP tests.
- Added explicit semver tag-format guard in release deploy script for fail-fast behavior in reused/manual contexts.
- Applied rate-limit parity to registrar routes and browser results route.
- Tightened API mutation contract (explicit JSON content-type + unknown-field rejection).
- Preferred fail-fast CLI/config validation over silent fallback behavior.
- Active backlog rows now keep only unresolved/externally-dependent items; completed/check work is folded into `Recently Closed IDs` to keep queue readable.
- Sonar queue reporting now uses strict OPEN filter (`projects=SaveEnergy_openbyte`, `issueStatuses=OPEN`) to match Cloud totals.

### Verification Baseline

- `go test ./cmd/check ./cmd/mcp ./cmd/server ./cmd/client`
- `go test ./test/unit/api ./test/unit/client ./test/unit/mcp ./test/unit/results ./test/unit/websocket`
- `go test ./internal/results`

### Test Layout Note

- Preferred location: `test/` tree.
- Exception: legacy white-box tests still co-located under `cmd/` and `internal/` where package-private access is required.
- Newly added rogue tests were moved to `test/unit/`.

## Open / Deferred

- Public hosted test fleet (infra/cost decision).
- Additional SDKs from OpenAPI (TypeScript/Python).
- Packaging/distribution polish (Homebrew/apt repos).
