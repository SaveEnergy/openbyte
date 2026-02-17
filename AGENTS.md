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
- Keep entries attributable (`Agent`, `Evidence`, `Check`).
- Resolve overlaps explicitly in `Decision Notes` (no silent overwrite).

### Live Queue (active only)

| ID | Area | Agent | Status | Plan | Evidence | Check |
| --- | --- | --- | --- | --- | --- | --- |
| 20260217-sec-01 | security | A0 | Check | Close strict `SECURITY` findings in `cmd/loadtest/main.go`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`. | Implemented hardening: removed `InsecureSkipVerify` in `cmd/loadtest/main.go` and moved workflow-level permissions to per-job scopes in `ci.yml` and `release.yml`; awaiting Sonar rescan to close stale OPEN entries. | `go test -short ./cmd/loadtest` |
| 20260217-web-03 | web | A0 | In Progress | Burn down JS maintainability hotspots in `web/openbyte.js`, `web/download.js`, `web/results.js`, `web/skill.js` (`javascript:S3776`, `S2486`, `S3504`, `S2004`, `S7762`). | Batch-6 implemented: extracted release fetch/fallback helpers in `web/download.js` (`fetchLatestRelease`, `applyGithubFallback`) and flattened startup fetch error handling; prior Batch-4/5 complexity reductions in `web/openbyte.js` remain in place. | `npx prettier --check web/*.js && bunx playwright test` |
| 20260217-go-10 | go | A0 | In Progress | Reduce remaining production complexity/literal hotspots in `cmd/server/main.go`, `internal/api/speedtest.go`, `internal/stream/server.go`, `internal/websocket/server.go`, `cmd/check/main.go`. | Batch-3 implemented in `cmd/server/main.go`: extracted runtime resource lifecycle into `serverResources` + `setupRuntimeResources`/`stopAll`, reducing `Run` branching and centralizing startup/shutdown paths; Batch-2 `internal/api/speedtest.go` upload helper split remains applied. | `go test -short ./cmd/server ./internal/api ./internal/stream ./internal/websocket ./cmd/check` |
| 20260217-go-11 | client | A0 | In Progress | Clear client-path residuals in `pkg/client/client.go`, `cmd/client/{cli,config,engine,main,run}.go`, `cmd/client/formatter.go` (`go:S3776`, `go:S1186`). | Batch-1 implemented in `cmd/client/run.go`: extracted shared helpers (`cancelStreamWithCleanup`, `computeProgress`, `emitProgressAndMetrics`, `finalizeHTTPStreamRun`) and simplified `runStream`, `runClientSideTest`, and `runHTTPStream` control flow while preserving behavior. | `go test -short ./cmd/client ./pkg/client ./test/unit/client` |
| 20260217-test-08 | test | A0 | In Progress | Continue test literal cleanup (`go:S1192`) in `test/e2e/e2e_test.go`, `test/unit/api/{router,clientip,speedtest,handlers}_test.go`, `test/unit/results/store_test.go`, `test/unit/types/stream_test.go`. | Batch-2 implemented: added/reused shared constants in `test/unit/api/speedtest_test.go`, `test/unit/api/router_test.go`, and `test/unit/api/clientip_test.go` (content type/cache control/API paths/loopback IP) to reduce remaining literal duplication while preserving assertions. | `go test -short ./test/e2e ./test/unit/api ./test/unit/results ./test/unit/types` |
| 20260217-test-09 | test | A0 | In Progress | Finish mechanical `godre:S8193` cleanup in `cmd/*_test.go`, `internal/*_test.go`, and remaining low-risk runtime hits. | Batch-2 implemented: removed remaining condition-scope `err` declarations in `internal/api/handlers_internal_test.go`, `test/unit/websocket/server_test.go`, `test/unit/client/api_test.go`, `cmd/client/cli_test.go` plus low-risk runtime tails in `cmd/check/main.go`, `internal/api/speedtest.go`, and `internal/websocket/server.go`; keep task open pending Sonar rescan confirmation. | `go test -short ./cmd/check ./cmd/loadtest ./cmd/server ./internal/api ./internal/stream ./internal/websocket ./test/unit/...` |
| 20260217-test-10 | test | A0 | In Progress | Resolve empty-method/code-smell findings (`go:S1186`) in `cmd/client/{run_test.go,api_internal_test.go,formatter.go}`, plus minor leftovers in `cmd/server/perf.go`, `internal/metrics/collector.go`. | Batch-2 implemented: replaced the disabled-stats empty closure path in `cmd/server/perf.go` with explicit no-op statement body; client formatter/test no-op updates and `Collector.Close()` cleanup from Batch-1 remain in place. | `go test -short ./cmd/client ./cmd/server ./internal/metrics ./test/unit/client` |

### Sonar Snapshot (latest recheck)

- Strict OPEN filter parity maintained with Cloud:
  - Query: `projects=[SaveEnergy_openbyte]`, `issueStatuses=[OPEN]`, `ps=500`
  - Total OPEN: `201`
  - Current top tracked rules: `go:S1192=79`, `godre:S8193=22`, `go:S3776=19`, `go:S1186=18`
  - Security OPEN: `5` (`go:S4830=1`, `go:S5527=1`, `githubactions:S8233=2`, `githubactions:S8264=1`)

### Recently Closed IDs

- Most historical IDs intentionally pruned for readability; canonical record remains in git history.
- Recent close: `20260217-ci-09`.
- Latest completed wave (moved `Check -> Done -> removed`):
  - `20260217-web-02`, `20260217-go-02`, `20260217-go-03`, `20260217-go-04`, `20260217-go-05`, `20260217-go-06`, `20260217-go-07`, `20260217-go-08`, `20260217-go-09`
  - `20260217-test-02`, `20260217-test-03`, `20260217-test-04`, `20260217-test-05`, `20260217-test-06`, `20260217-test-07`

### Recent Decision Notes

- Adopted Go 1.26 baseline across runtime and CI/release workflows.
- Sonar reporting uses strict OPEN parity query (`projects=SaveEnergy_openbyte`, `issueStatuses=OPEN`).
- Prefer behavior-preserving refactors + targeted regression tests over broad rewrites.
- Active backlog rows now keep only unresolved/externally-dependent items; completed/check work is folded into `Recently Closed IDs` to keep queue readable.

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
