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
| 20260217-test-08 | test | A0 | Done | Continue test literal cleanup (`go:S1192`) in `test/e2e/e2e_test.go`, `test/unit/api/{router,clientip,speedtest,handlers}_test.go`, `test/unit/results/store_test.go`, `test/unit/types/stream_test.go`. | Batch-108 closure: retained and extended literal dedups (including integration assertions), with full scope gate passing in this run. | `go test -count=3 -short ./test/e2e ./test/unit/api ./test/unit/results ./test/unit/types && go test -short ./test/integration` |
| 20260217-web-03 | web | A0 | Done | Burn down JS maintainability hotspots in `web/openbyte.js`, `web/download.js`, `web/results.js`, `web/skill.js` (`javascript:S3776`, `S2486`, `S2004`, `S7766`). | Batch-10 closure: completed module split and follow-up runtime fix (`SVG class` assignment in `web/ui.js`), with targeted failing flow tests and full UI suite green. | `npx prettier --check web/*.js && bunx playwright test` |
| 20260218-web-04 | web | A0 | Done | Close frontend readability/consistency findings outside current complexity track: `web/openbyte.js` (`javascript:S107`, `S7735`), `web/style.css` (`css:S4666`), `test/e2e/ui/basic.spec.js` (`javascript:S7773`). | Batch-6 closure: readability/consistency refactors retained after module extraction, formatting clean, and end-to-end UI suite passing. | `npx prettier --check web/*.js test/e2e/ui/*.js && bunx playwright test` |
| 20260226-web-07 | web | A0 | Done | Close remaining low-count Sonar web rules not yet mapped in active rows: `Web:S6819`, `javascript:S1874`, `javascript:S6582`. | Batch-3 closure: semantic element remediations (`<progress>`, `<output>`) and JS updates validated by full UI suite; Sonar server parity expected on next external reanalysis. | `npx prettier --check web/index.html web/results.html web/style.css web/openbyte.js && bunx playwright test` |
| 20260226-go-23 | go | A0 | Done | Close residual low-count Go reliability/code-smell rule `godre:S8196` with targeted fix and regression coverage. | Batch-2 closure: `RouteRegistrar` renamed to `RoutesRegistrar` and consumer updated; focused and aggregate Go gates pass. | `go test -short ./internal/api ./cmd/server && go build ./...` |
| 20260219-go-14 | go | A2 | Done | Split oversized Go production files to respect 500 LOC rule: `internal/api/handlers.go` (629 LOC), `internal/stream/server.go` (548), `cmd/client/engine.go` (541), `internal/api/router.go` (500). | Batch-2 closure: split into `handlers_http.go`, `server_udp.go`, and `engine_direction.go`; resulting target files all ≤500 LOC and compile/test gates green. | `wc -l internal/api/handlers.go internal/api/handlers_http.go internal/stream/server.go internal/stream/server_udp.go cmd/client/engine.go cmd/client/engine_direction.go && go build ./... && go test -short ./internal/api ./internal/stream ./cmd/client` |
| 20260219-web-01 | web | A0 | Done | Split `web/openbyte.js` monolith into focused ES modules (state, network, UI) to respect the 500 LOC/file rule. | Closure: split into focused modules (`state`, `utils`, `ui`, `network`, `settings`, `speedtest*`), entrypoint reduced to orchestration, all JS files ≤500 LOC, UI suite passing. | `wc -l web/*.js && npx prettier --check web/*.js && bunx playwright test` |
| 20260219-css-01 | web | A2 | Done | Split `web/style.css` monolith (1470 LOC) into modular partials: `base.css` (reset + variables + layout), page-specific files (`speed.css`, `download.css`, `results.css`, `skill.css`), and `motion.css`. | Closure: stylesheet modularized (`base`, `speed`, `download`, `modal`, `skill`, `motion`) with `style.css` as import entrypoint; all CSS files ≤500 LOC. | `wc -l web/*.css && bunx playwright test` |
| 20260220-perf-01 | web | A2 | Done | Web performance: add `<link rel="preload">` for critical fonts (96 KB woff2, 4 files), add `defer` to all `<script>` tags, add HTTP compression middleware (gzip/brotli) for embedded assets. | Closure: font preloads present on all pages, scripts deferred/module-loaded, and gzip middleware active on static assets. | `curl -I -H 'Accept-Encoding: gzip' http://127.0.0.1:8080/speed.css` shows `Content-Encoding: gzip` |

### Check Hold (manual/external)

| ID | Area | Agent | Status | Plan | Evidence | Check |
| --- | --- | --- | --- | --- | --- | --- |
| 20260217-go-10 | go | A0 | Done | Reduce remaining production complexity/literal hotspots in `cmd/server/main.go`, `internal/api/speedtest.go`, `internal/stream/server.go`, `internal/websocket/server.go`, `cmd/check/main.go`. | Closure verification: race gate re-run in this pass after structural splits; no regressions in target packages. | `go test -race -short ./cmd/server ./internal/api ./internal/stream ./internal/websocket ./cmd/check` |
| 20260217-go-11 | client | A0 | Done | Clear client-path residuals in `pkg/client/client.go`, `cmd/client/{cli,config,engine,main,run}.go`, `cmd/client/formatter.go` (`go:S3776`, `go:S1186`). | Closure verification: race gate re-run in this pass and client suites remain green after engine file split. | `go test -race -short ./cmd/client ./pkg/client ./test/unit/client` |
| 20260219-a11y-01 | a11y | A2 | Done | Remediate accessibility gaps: add skip-to-content links on all 4 HTML pages, add `aria-busy` on `#testingState` during test execution, add `role="progressbar"` + `aria-valuenow` on progress ring SVG, add `<main>` landmark to `download.html`/`results.html`/`skill.html`, verify `<dialog>` focus trap cross-browser. | Batch-3 closure: semantic progress/loading elements migrated to native controls, skip links/landmarks and busy state retained, and full UI regression suite green after modal + state flow checks. | `bunx playwright test` |

### Sonar Snapshot (latest recheck)

- Strict OPEN filter parity maintained with Cloud:
  - Query: `projects=[SaveEnergy_openbyte]`, `issueStatuses=[OPEN]`, `ps=500`
  - Total OPEN: `81`
  - Current top tracked rules: `go:S1192=49`, `go:S3776=14`, `javascript:S3776=4`, `javascript:S2004=3`, `Web:S6819=2`, `javascript:S7735=2`, `javascript:S2486=2`, `javascript:S7766=2`, `javascript:S1874=1`, `javascript:S6582=1`, `godre:S8196=1` (MCP live fetch on 2026-02-26; project `SaveEnergy_openbyte`)
  - Rule-to-backlog mapping refreshed:
    - `go:S1192` -> `20260217-test-08`
    - `go:S3776` -> `20260217-go-10`, `20260217-go-11`
    - `javascript:S3776`, `javascript:S2004`, `javascript:S2486`, `javascript:S7766` -> `20260217-web-03`
    - `javascript:S7735` -> `20260218-web-04`
    - `Web:S6819`, `javascript:S1874`, `javascript:S6582` -> `20260226-web-07`
    - `godre:S8196` -> `20260226-go-23`
  - Security OPEN: `0`

### Recently Closed IDs

- Most historical IDs intentionally pruned for readability; canonical record remains in git history.
- Recent close: `20260217-ci-09`.
- Latest completed wave (moved `Check -> Done -> removed`):
  - `20260217-web-02`, `20260217-go-02`, `20260217-go-03`, `20260217-go-04`, `20260217-go-05`, `20260217-go-06`, `20260217-go-07`, `20260217-go-08`, `20260217-go-09`
  - `20260217-test-02`, `20260217-test-03`, `20260217-test-04`, `20260217-test-05`, `20260217-test-06`, `20260217-test-07`
  - `20260217-sec-01`, `20260218-go-12`, `20260218-go-13`, `20260219-ui-01`, `20260219-ui-02`, `20260219-web-02`, `20260219-web-05`, `20260219-web-06`, `20260219-ui-03`, `20260219-cli-03`, `20260219-go-16`, `20260219-cli-01`, `20260219-cli-02`, `20260219-ui-04`, `20260219-ui-05`, `20260219-go-15`, `20260217-test-09`, `20260217-test-10`, `20260219-go-17`, `20260219-go-18`, `20260219-go-19`, `20260219-ci-01`, `20260219-doc-01`, `20260219-ui-06`, `20260219-ui-07`, `20260219-go-20`, `20260219-go-21`, `20260220-sec-01`, `20260220-api-01`, `20260219-go-22`, `20260220-web-01`, `20260220-meta-01`, `20260219-sdk-01`, `20260219-reg-01`, `20260219-test-13`, `20260219-test-11`, `20260219-test-12`

### Recent Decision Notes

- Adopted Go 1.26 baseline across runtime and CI/release workflows.
- Sonar reporting uses strict OPEN parity query (`projects=SaveEnergy_openbyte`, `issueStatuses=OPEN`).
- Prefer behavior-preserving refactors + targeted regression tests over broad rewrites.
- Active backlog rows now keep only unresolved/externally-dependent items; completed/check work is folded into `Recently Closed IDs` to keep queue readable.
- A2 full-codebase analysis (2026-02-19): identified 9 new backlog items across a11y, CSS architecture, Go LOC splits, test coverage gaps, flaky tests, and frontend UX. Sonar snapshot updated with per-rule breakdown (133 OPEN). Key risk: `cmd/client/engine.go` has zero direct test coverage (541 LOC). UDP `lastSeenUnix` concurrency verified safe (atomic ops). `<dialog>` focus trap assumption needs cross-browser validation.
- A2 second-pass analysis (2026-02-19): deep-dive into CI/CD, error handling, SDK surface, registry resilience, and frontend runtime. Added 6 new items: error wrapping (`%v`→`%w`), config init hardening (hostname/CIDR), SDK thread-safety docs, registry backoff/jitter, CI pin gaps, deploy docs. Verified false positives: JS single-threaded rules out `startTest` race; `progressTick` cleared in `finally`; `fetchWithTimeout` abort listener properly cleaned via `.finally()`; Alpine BusyBox includes `wget` (health check OK); `TEST_CONFIG` already centralizes most magic numbers. Frontend `state.ws` field confirmed dead code.
- A1 correctness and reliability pass (2026-02-19): Deep dive on core stream, metrics, and result storage concurrency/invariants. Fixed unreachable return statements masking loop termination intents in `internal/results/store.go` SQLite retry loops. Found and fixed a data race in `internal/metrics/collector.go` where `latencyHistogram` was zeroed during `Close()` while concurrently read without a lock by `RecordLatency`. Verified UDP active counts and server context cancellation limits as safe.
- A1 third-pass analysis (2026-02-19): frontend accessibility and backend performance critique. Identified WCAG 1.4.4 text scaling barrier in `web/style.css` (`font-size: 16px` on `html`) and screen-reader overwhelming via 100ms `aria-valuenow` `setInterval` updates in `web/openbyte.js`. Found high GC allocation path in `internal/api/speedtest.go` (256KB per HTTP upload via `readUploadBody`) missing `sync.Pool`, and excessive CPU wakeups from per-download stream `100ms` tickers (up to 1,000 wakeups/sec under load). Items added to active queue.
- A1 fourth-pass analysis (2026-02-19): Deep dive into Web UI edge cases and Go WebSocket behavior. Identified a persistent memory leak in `internal/websocket/server.go` where the server relies entirely on the client to close terminal connections. Found a critical Safari UX bug in `web/openbyte.js` where `await`ing the result save before calling `navigator.clipboard.writeText` causes a `NotAllowedError` by breaking the synchronous user-gesture context. Noted missing background scroll lock (`overflow: hidden`) during modal presentation, causing scroll chaining on mobile. Added to active queue.
- A2 third-pass analysis (2026-02-20): security deep-dive, web performance, visual critique (browser), dependency health, cross-page consistency. Security posture strong: CORS dot-boundary safe, rate-limiter memory-bounded + timing-safe, path traversal blocked by allowlist + Clean + `..` check, share ID crypto-secure (`crypto/rand` + rejection sampling), API key constant-time compare. `govulncheck` clean; all deps current. Added 5 new items: CSP `'unsafe-inline'` removal (10 inline attrs convertible to CSS classes), API JSON 404, web perf (font preload/defer/compression), meta/SEO/footer consistency, skill page responsive tables + dead SVG cleanup.

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
