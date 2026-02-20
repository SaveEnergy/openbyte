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
| 20260217-go-10 | go | A0 | In Progress | Reduce remaining production complexity/literal hotspots in `cmd/server/main.go`, `internal/api/speedtest.go`, `internal/stream/server.go`, `internal/websocket/server.go`, `cmd/check/main.go`. | Batch-4 implemented (PDCA cycle): closed remaining low-risk mechanical tails across tracked runtime surfaces — grouped `parseDuration` params in `cmd/server/main.go` (`godre:S8209`), grouped `NewSpeedTestHandler` params + simplified atomic limit checks in `internal/api/speedtest.go`, simplified atomic limit checks in `internal/stream/server.go`, and removed condition-scope temp in `cmd/check/main.go`. Targeted suites green. | `go test -short ./cmd/server ./internal/api ./internal/stream ./internal/websocket ./cmd/check` |
| 20260217-go-11 | client | A0 | In Progress | Clear client-path residuals in `pkg/client/client.go`, `cmd/client/{cli,config,engine,main,run}.go`, `cmd/client/formatter.go` (`go:S3776`, `go:S1186`). | Batch-2 implemented: reduced complexity in `cmd/client/config.go` by flattening `applyFlagOverrides` through typed override helpers; reduced complexity in `cmd/client/cli.go` by splitting `selectFastestServer` into `collectServerLatencies`/`probeServerLatency`/`pickFastestServer`/`printServerLatencies`; behavior preserved, tests green. | `go test -short ./cmd/client ./pkg/client ./test/unit/client` |
| 20260217-web-03 | web | A0 | In Progress | Burn down JS maintainability hotspots in `web/openbyte.js`, `web/download.js`, `web/results.js`, `web/skill.js` (`javascript:S3776`, `S2486`, `S3504`, `S2004`, `S7762`). | Batch-8 implemented: flipped negated warm-up branches in `web/openbyte.js` download/upload progress accounting to positive-first flow (targeting `S7735` noise reduction) while preserving behavior; prior Batch-4/5/6/7 changes remain in place. | `npx prettier --check web/*.js && bunx playwright test` |
| 20260218-web-04 | web | A0 | In Progress | Close frontend readability/consistency findings outside current complexity track: `web/openbyte.js` (`javascript:S107`, `S7735`), `web/style.css` (`css:S4666`), `test/e2e/ui/basic.spec.js` (`javascript:S7773`). | Batch-4 implemented: `Number.parseFloat` migration in `test/e2e/ui/basic.spec.js`; removed duplicate `.download-meta`/`.download-link` selectors in `web/style.css`; refactored `web/openbyte.js` to convert long-arg functions (`tryDownloadChunkWithRetries`, `runSingleUploadStream`) to options objects and flipped warm-up negated branches. Awaiting Sonar rescan for residual `S7735`. | `npx prettier --check web/*.js test/e2e/ui/*.js && bunx playwright test` |
| 20260219-a11y-01 | a11y | A2 | Check | Remediate accessibility gaps: add skip-to-content links on all 4 HTML pages, add `aria-busy` on `#testingState` during test execution, add `role="progressbar"` + `aria-valuenow` on progress ring SVG, add `<main>` landmark to `download.html`/`results.html`/`skill.html`, verify `<dialog>` focus trap cross-browser. | Batch-1 implemented: added `.skip-link` + `#mainContent` anchors across `web/index.html`, `web/download.html`, `web/results.html`, `web/skill.html`; added `aria-busy` state handling on `#testingState` in `web/openbyte.js`; added progress semantics (`role="progressbar"`, `aria-valuemin/max/now`) via `#progressMeter` in `web/index.html` with live `aria-valuenow` updates in JS. UI suite green; manual keyboard/screen-reader validation still pending for cross-browser `<dialog>` focus behavior. | Manual keyboard + screen reader test on Chrome/Firefox/Safari; `bunx playwright test` |
| 20260219-go-15 | go | A2 | Check | Clear remaining minor `godre` Sonar findings: `S8209` (4 hits), `S8184` (3), `S8196` (2), `S8159` (1), `S8242` (1) — 11 issues total, all mechanical. | Batch-6 implemented: retained prior context-field/interface/import cleanups; added blank-import rationale comments in `internal/results/store.go` and `test/unit/results/store_test.go` (`godre:S8184`), plus additional `S8209/S8193` cleanup in `internal/api/speedtest.go` and `internal/stream/server.go`. Targeted runtime + unit suites green; awaiting Sonar rescan parity for final close. | `go build ./...`; `go test -short ./...` |
| 20260217-test-08 | test | A0 | In Progress | Continue test literal cleanup (`go:S1192`) in `test/e2e/e2e_test.go`, `test/unit/api/{router,clientip,speedtest,handlers}_test.go`, `test/unit/results/store_test.go`, `test/unit/types/stream_test.go`. | Batch-3 implemented (PDCA cycle): expanded constant reuse in `test/unit/api/router_test.go` (`Access-Control-Allow-Origin`, `Cache-Control`, `no-store`, origin URLs, results DB path/error format) and `test/e2e/e2e_test.go` (`127.0.0.1`, `/openbyte.js`, repeated marshal/start error formats, `no-store` usage) to reduce duplicated literals while preserving behavior. Target suites green. | `go test -short ./test/e2e ./test/unit/api ./test/unit/results ./test/unit/types` |
| 20260217-test-09 | test | A0 | Check | Finish mechanical `godre:S8193` cleanup in `cmd/*_test.go`, `internal/*_test.go`, and remaining low-risk runtime hits. | Batch-3 implemented (PDCA cycle): removed remaining condition-scope temporary variables in `internal/stream/server_internal_test.go`, `test/unit/api/router_test.go`, and `test/unit/api/speedtest_test.go`, plus runtime tails in `cmd/check/main.go`, `internal/api/speedtest.go`, and `internal/stream/server.go`. Target suites green; pending Sonar parity refresh. | `go test -short ./cmd/check ./cmd/loadtest ./cmd/server ./internal/api ./internal/stream ./test/unit/api ./test/unit/results` |
| 20260217-test-10 | test | A0 | Check | Resolve empty-method/code-smell findings (`go:S1186`) in `cmd/client/{run_test.go,api_internal_test.go,formatter.go}`, plus minor leftovers in `cmd/server/perf.go`, `internal/metrics/collector.go`. | Batch-3 verification: prior no-op-body and collector cleanups retained; latest Sonar OPEN snapshot shows no residual `go:S1186` entries. Keep at Check until next parity pull confirms closure persistence after recent commits. | `go test -short ./cmd/client ./cmd/server ./internal/metrics ./test/unit/client` |
| 20260219-test-13 | test | A2 | Planned | Remediate time-dependent flaky test patterns: `test/unit/api/ratelimit_test.go` uses `time.Sleep(2-6s)`, `test/unit/registry/client_test.go` uses `waitFor()` with 500-800ms timeouts. | Sleep-based synchronization is inherently flaky under CI load; inject clock interface or use deterministic time source. | `go test -count=5 -short ./test/unit/api ./test/unit/registry` stable |
| 20260219-test-11 | test | A2 | Planned | Add direct unit tests for `cmd/client/engine.go` TCP/UDP proxy engine — critical untested production path (541 LOC, 0 coverage). | No `engine_test.go` or `test/unit/client/engine_test.go` exists; engine is only tested indirectly via E2E. Handles download/upload/bidirectional proxy with connection management. | `go test -short ./cmd/client` covers new tests; `-race` clean |
| 20260219-web-01 | web | A0 | Planned | Split `web/openbyte.js` monolith into focused ES modules (state, network, UI) to respect the 500 LOC/file rule. | `web/openbyte.js` is ~1660 lines, mixing HTTP execution, DOM binding, and application state. | `bunx playwright test` |
| 20260219-css-01 | web | A2 | Planned | Split `web/style.css` monolith (1470 LOC) into modular partials: `base.css` (reset + variables + layout), page-specific files (`speed.css`, `download.css`, `results.css`, `skill.css`), and `motion.css`. | `style.css` is 1470 lines, 3× the 500 LOC rule; mixes 4 pages' styles, reset, variables, breakpoints, and keyframes in one file. | `wc -l web/*.css` all ≤500; `bunx playwright test` |
| 20260219-go-14 | go | A2 | Planned | Split oversized Go production files to respect 500 LOC rule: `internal/api/handlers.go` (629 LOC), `internal/stream/server.go` (548), `cmd/client/engine.go` (541), `internal/api/router.go` (500). | `wc -l` confirms 4 files at or above limit; `handlers.go` can split by handler group (results/stream/health), `stream/server.go` by protocol (TCP/UDP), `engine.go` by direction, `router.go` by middleware vs route registration. | `wc -l` on split files all ≤500; `go build ./...`; `go test -short ./internal/api ./internal/stream ./cmd/client` |
| 20260219-test-12 | test | A2 | Planned | Split oversized test files: `test/unit/api/handlers_test.go` (1140 LOC) by handler group, `test/e2e/e2e_test.go` (740 LOC) by feature area. | Both exceed 500 LOC rule; `handlers_test.go` is 2.3× limit. | `wc -l` on split files all ≤500; `go test -short ./test/unit/api ./test/e2e` |
| 20260219-cli-01 | cli | A1 | Planned | Remove secret flags (`--api-key`, `--registry-api-key`) across all commands to prevent credential leakage in process lists. | `cli/SKILL.md` rules broken. `cmd/server/main.go`, `cmd/client/cli.go`, `cmd/check/main.go` pass secrets via command line flags. | `openbyte client -h` shows no api-key flag; env var loading intact. |
| 20260219-cli-02 | cli | A1 | Planned | Standardize `-S` flag behavior across `client` and `check` commands to map consistently. | Currently `-S` means `server` alias in `client` but `server-url` in `check`. | Code review of CLI initialization functions. |
| 20260219-go-16 | go | A2 | Planned | Fix error wrapping: 3 sites in `cmd/client/run.go` use `%v` instead of `%w` (lines 21, 129, 230), breaking `errors.Is()`/`errors.As()` unwrapping. | `run.go:21` wraps stream start error with `%v`; `run.go:129` wraps combined run+complete errors with `%v`; `run.go:230` wraps cancel cleanup errors with `%v`. All should use `%w` to preserve error chains. | `go test -short ./cmd/client`; `grep -n '%v' cmd/client/run.go` shows 0 non-format uses |
| 20260219-go-17 | go | A2 | Planned | Harden config init: `internal/config/config.go:73` silently ignores `os.Hostname()` failure (empty `ServerID`); `internal/api/clientip.go:85` silently skips invalid CIDR entries (should fail fast or return error during init). | `hostname, _ := os.Hostname()` — empty ServerID causes identification issues. `parseTrustedProxyCIDRs` logs Warning but continues, meaning misconfigured proxy trust is invisible. | `go test -short ./internal/api ./internal/config ./test/unit/config` |
| 20260219-sdk-01 | sdk | A2 | Planned | Document `pkg/client.Client` thread safety (safe for concurrent calls if not mutated after creation); add tool name + input context to MCP error messages in `cmd/mcp/main.go`. | `Client` struct fields (`serverURL`, `httpClient`, `apiKey`) unprotected by mutex — safe only if immutable after `New()`. MCP errors use generic format strings without tool/input context, making debugging difficult. | `go test -short ./pkg/client ./cmd/mcp ./test/unit/mcp` |
| 20260219-reg-01 | reliability | A2 | Planned | Add exponential backoff + jitter to registry client heartbeat loop (`internal/registry/client.go:216-233`); currently retries at fixed interval with no backoff, and all servers may heartbeat simultaneously. | `heartbeatLoop` uses fixed `time.Ticker`; errors logged but no backoff. Initial registration failure (line 73) doesn't prevent heartbeat start, causing inconsistent state. No jitter → thundering herd risk with multi-server deployments. | `go test -short ./internal/registry ./test/unit/registry` |
| 20260219-ci-01 | ci | A2 | Planned | Pin remaining unpinned CI/deploy images: `actions/cache@v5` (no SHA pin in `.github/workflows/ci.yml`), `traefik:v3` unpinned in `docker/docker-compose.traefik.yaml` and `docker-compose.ghcr.traefik.yaml`. | All other Actions are SHA-pinned; `cache@v5` is the sole exception. Traefik `v3` tag is mutable — major version bump could break reverse proxy config silently. | `grep -c 'actions/cache@v5' .github/workflows/*.yml` returns 0; `grep 'traefik:v3\.' docker/*.yaml` shows pinned versions |
| 20260219-doc-01 | docs | A2 | Planned | Add `.env.example` deployment template (deploy scripts expect `.env` at `REMOTE_DIR/.env` but no template exists); document WebSocket upgrade endpoint in `api/openapi.yaml` (currently only shows 101 response, no protocol details). | Deploy workflow (`ci.yml:226+`) references remote `.env`; new deployers have no variable reference. OpenAPI `paths[/api/v1/stream/{id}/stream]` lacks WebSocket protocol upgrade documentation. | `.env.example` exists with documented vars; OpenAPI WebSocket path has `x-websocket` or upgrade detail |
| 20260219-go-18 | go | A1 | Check | Remove dead code in `internal/results/store.go` busy-retry loops. | Replaced unreachable `return` statements with `panic("unreachable")` outside `for` loops in `insertResultWithRetry`, `execWithBusyRetry`, and `Get` to satisfy compiler while explicitly marking dead code. | `go test -short ./internal/results` |
| 20260219-go-19 | go | A1 | Check | Fix data race on `latencyHistogram` access in `internal/metrics/collector.go`. | Modified `Collector.Close()` to not nil out `c.latencyHistogram`, preventing concurrent read races in `RecordLatency` which reads it outside the mutex for performance. | `go test -race ./internal/metrics` |

### Sonar Snapshot (latest recheck)

- Strict OPEN filter parity maintained with Cloud:
  - Query: `projects=[SaveEnergy_openbyte]`, `issueStatuses=[OPEN]`, `ps=500`
  - Total OPEN: `133`
  - Current top tracked rules: `go:S1192=70`, `go:S3776=16`, `godre:S8193=8`, `javascript:S3776=5`, `godre:S8209=5`, `javascript:S7735=4`, `godre:S8184=3`, `css:S4666=3`, `javascript:S107=2`, `go:S107=2`, `javascript:S7773=2` (latest MCP fetch; Sonar server-side issue recalculation may lag behind local fixes)
  - Security OPEN: `0`

### Recently Closed IDs

- Most historical IDs intentionally pruned for readability; canonical record remains in git history.
- Recent close: `20260217-ci-09`.
- Latest completed wave (moved `Check -> Done -> removed`):
  - `20260217-web-02`, `20260217-go-02`, `20260217-go-03`, `20260217-go-04`, `20260217-go-05`, `20260217-go-06`, `20260217-go-07`, `20260217-go-08`, `20260217-go-09`
  - `20260217-test-02`, `20260217-test-03`, `20260217-test-04`, `20260217-test-05`, `20260217-test-06`, `20260217-test-07`
  - `20260217-sec-01`, `20260218-go-12`, `20260218-go-13`, `20260219-ui-01`, `20260219-ui-02`, `20260219-web-02`, `20260219-web-05`, `20260219-web-06`, `20260219-ui-03`, `20260219-cli-03`

### Recent Decision Notes

- Adopted Go 1.26 baseline across runtime and CI/release workflows.
- Sonar reporting uses strict OPEN parity query (`projects=SaveEnergy_openbyte`, `issueStatuses=OPEN`).
- Prefer behavior-preserving refactors + targeted regression tests over broad rewrites.
- Active backlog rows now keep only unresolved/externally-dependent items; completed/check work is folded into `Recently Closed IDs` to keep queue readable.
- A2 full-codebase analysis (2026-02-19): identified 9 new backlog items across a11y, CSS architecture, Go LOC splits, test coverage gaps, flaky tests, and frontend UX. Sonar snapshot updated with per-rule breakdown (133 OPEN). Key risk: `cmd/client/engine.go` has zero direct test coverage (541 LOC). UDP `lastSeenUnix` concurrency verified safe (atomic ops). `<dialog>` focus trap assumption needs cross-browser validation.
- A2 second-pass analysis (2026-02-19): deep-dive into CI/CD, error handling, SDK surface, registry resilience, and frontend runtime. Added 6 new items: error wrapping (`%v`→`%w`), config init hardening (hostname/CIDR), SDK thread-safety docs, registry backoff/jitter, CI pin gaps, deploy docs. Verified false positives: JS single-threaded rules out `startTest` race; `progressTick` cleared in `finally`; `fetchWithTimeout` abort listener properly cleaned via `.finally()`; Alpine BusyBox includes `wget` (health check OK); `TEST_CONFIG` already centralizes most magic numbers. Frontend `state.ws` field confirmed dead code.
- A1 correctness and reliability pass (2026-02-19): Deep dive on core stream, metrics, and result storage concurrency/invariants. Fixed unreachable return statements masking loop termination intents in `internal/results/store.go` SQLite retry loops. Found and fixed a data race in `internal/metrics/collector.go` where `latencyHistogram` was zeroed during `Close()` while concurrently read without a lock by `RecordLatency`. Verified UDP active counts and server context cancellation limits as safe.

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
