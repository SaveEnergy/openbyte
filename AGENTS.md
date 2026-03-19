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
- JSON API handlers enforce size limits and single-object decoding for POST/PUT payloads (`internal/jsonbody.DecodeSingleObject` shared by API + results).
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
- Speed test UI: primary logic in `speedtest*.js`; `openbyte.js` wires orchestration and server selection—keep new probe/state changes localized to those modules.

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
| _none_ | - | - | - | No active queued rows. | `20260319-refactor-01`..`06` landed 2026-03-19 (see Decision Notes). | N/A |

### Check Hold (manual/external)

| ID | Area | Agent | Status | Plan | Evidence | Check |
| --- | --- | --- | --- | --- | --- | --- |
| _none_ | - | - | - | No pending manual/external checks. | Last wave completed with local verification. | N/A |

### Sonar Snapshot (latest recheck)

- Strict OPEN filter parity maintained with Cloud:
  - Query: `projects=[SaveEnergy_openbyte]`, `issueStatuses=[OPEN]`, `ps=500`
  - Total OPEN: `0` (MCP live fetch on 2026-03-01; all issues closed)
  - Previous snapshot: `23` (2026-02-26); all resolved via sonar-07/08/09 closure wave
  - Current top tracked rules: none (all `0`)
  - Rule-to-backlog mapping refreshed:
    - `go:S3776`, `godre:S8196` -> closed in `20260226-sonar-07` (pending remote rescan parity)
    - `go:S1192` -> closed in `20260226-sonar-08` (pending remote rescan parity)
    - `javascript:S1854`, `javascript:S1481`, `javascript:S3358`, `javascript:S3776`, `javascript:S7785`, `javascript:S7787` -> closed in `20260226-sonar-09` (pending remote rescan parity)
    - CI frontend-contract regressions from modularization -> closed in `20260226-ci-10`
    - Security hotspots (`security_hotspots`, `new_security_hotspots`) -> `20260226-sec-02`
    - Security quality remains clear (`security issues=0`, `vulnerabilities=0`, hotspots `0/100% reviewed`)
  - Security OPEN issues: `0`
  - Security hotspot debt: `0` total, `0` new (`100%` reviewed overall)

### Recently Closed IDs

- Most historical IDs intentionally pruned for readability; canonical record remains in git history.
- Recent close: `20260319-refactor-01`..`06` (refactor wave).
- Latest completed wave (moved `Check -> Done -> removed`):
  - `20260319-refactor-01`, `20260319-refactor-02`, `20260319-refactor-03`, `20260319-refactor-04`, `20260319-refactor-05`, `20260319-refactor-06`
  - `20260228-sec-06`, `20260228-go-32`, `20260228-ui-09`, `20260228-go-33`, `20260301-web-07`, `20260301-a11y-02`, `20260301-ui-10`, `20260301-go-34`, `20260301-go-35`, `20260301-api-04`, `20260301-ws-02`, `20260301-ci-11`, `20260301-sec-07`, `20260301-web-06`, `20260301-web-08`, `20260301-ops-01`, `20260301-doc-02`
  - `20260217-web-02`, `20260217-go-02`, `20260217-go-03`, `20260217-go-04`, `20260217-go-05`, `20260217-go-06`, `20260217-go-07`, `20260217-go-08`, `20260217-go-09`
  - `20260217-test-02`, `20260217-test-03`, `20260217-test-04`, `20260217-test-05`, `20260217-test-06`, `20260217-test-07`
  - `20260217-sec-01`, `20260218-go-12`, `20260218-go-13`, `20260219-ui-01`, `20260219-ui-02`, `20260219-web-02`, `20260219-web-05`, `20260219-web-06`, `20260219-ui-03`, `20260219-cli-03`, `20260219-go-16`, `20260219-cli-01`, `20260219-cli-02`, `20260219-ui-04`, `20260219-ui-05`, `20260219-go-15`, `20260217-test-09`, `20260217-test-10`, `20260219-go-17`, `20260219-go-18`, `20260219-go-19`, `20260219-ci-01`, `20260219-doc-01`, `20260219-ui-06`, `20260219-ui-07`, `20260219-go-20`, `20260219-go-21`, `20260220-sec-01`, `20260220-api-01`, `20260219-go-22`, `20260220-web-01`, `20260220-meta-01`, `20260219-sdk-01`, `20260219-reg-01`, `20260219-test-13`, `20260219-test-11`, `20260219-test-12`, `20260226-sec-02`, `20260226-sonar-01`, `20260226-sonar-02`, `20260226-ci-10`, `20260226-go-24`, `20260226-go-25`, `20260226-go-26`, `20260226-sonar-03`, `20260226-api-02`, `20260226-web-03`, `20260226-go-04`, `20260226-web-04`, `20260226-sonar-04`, `20260226-sonar-05`, `20260226-sonar-06`, `20260226-sonar-07`, `20260226-sonar-08`, `20260226-sonar-09`, `20260226-perf-03`, `20260226-perf-05`, `20260226-perf-06`, `20260226-sec-03`, `20260226-sec-04`, `20260226-go-27`, `20260226-go-28`, `20260226-go-29`, `20260226-api-03`, `20260226-web-05`
- Marathon deferred/cancelled by design-risk guardrail: `20260226-perf-02`, `20260226-perf-04`.

### Recent Decision Notes

- 2026-03-19: Landed `20260319-refactor-01`..`06`: package `internal/jsonbody`; websocket files `origin.go`/`broadcast.go`/`limits.go`/`ping.go`/`lifecycle.go`/`message_types.go` + slim `server.go`; `speedtest_{download,upload,deadline}.go`; `handlers_meta.go`/`handlers_stream.go`; SDK `client_http.go`; `refactor-06` = AGENTS frontend ownership note only (no JS moves).
- 2026-03-19: Post-refactor gates green: `gofmt` on `internal/api/speedtest_download.go`, `make ci-lint`, `make ci-test`, Redocly lint + `TestOpenAPIRouteContract`, `go mod tidy` clean, `go test ./... -race -short -p 1`, full `bunx playwright test`.
- 2026-03-19: Security hygiene: `go 1.26.1`, indirect `github.com/buger/jsonparser v1.1.2` (Dependabot alert on transitive `mcp-go` → `jsonschema` → `go-ordered-map`), `docker/Dockerfile` builder `golang:1.26.1-alpine`; `govulncheck ./...` clean for reachable symbols.
- 2026-03-19: Dependency refresh: `golang.org/x/term v0.41.0`, `modernc.org/sqlite v1.47.0` (plus transitive `x/sys`, `modernc.org/libc`); supersedes open Dependabot PRs for those direct deps.
- 2026-03-19: Deep refactor analysis intake — Live Queue rows `20260319-refactor-01`..`06` (shared JSON decode, file splits for websocket/api/sdk, web module clarity); staged behavior-preserving refactors per Engineering Guardrails.
- Adopted Go 1.26.1 baseline (`go.mod`, Docker builder); CI uses `1.26.x` toolchain.
- Sonar reporting uses strict OPEN parity query (`projects=SaveEnergy_openbyte`, `issueStatuses=OPEN`).
- 2026-02-28 Sonar closure pass: implemented targeted fixes for rows `20260226-sonar-07/08/09` (Go complexity/naming, Go literals, web JS cleanup + module consistency) with green checks (`go test -short ./cmd/server ./cmd/client ./internal/stream ./internal/websocket ./internal/api ./test/e2e ./test/unit/metrics ./test/unit/api`, `npx prettier --check web/*.js`, `bunx playwright test`); awaiting next remote Sonar analysis for count parity.
- 2026-02-26 Sonar refresh (post progress-sync push): OPEN `23` (down from `29`), queue reopened with targeted rows `sonar-07/08/09` for residual Go + web clusters.
- Current Sonar MCP surface exposes issue search + metrics, but not hotspot-review transitions; hotspot closure requires Sonar UI/API support outside current MCP tools.
- 2026-02-26 parallel closure wave (4 subagents): closed all previously open live-queue rows (`ci-10`, `go-24`, `go-25`, `go-26`, `sonar-03`, `api-02`, `web-03`, `go-04`, `web-04`) with green local gates (`make ci-lint`, `go test -short ./...`, `bunx playwright test`); historical snapshot at that checkpoint was `29` OPEN.
- 2026-02-26 Sonar refresh (post-push): OPEN remains `29` with shifted composition; targeted rows `sonar-04/05/06` executed in marathon wave with local gates green (remote Sonar parity pending next analysis).
- 2026-02-26 Fast.com research intake: added performance backlog wave (`perf-02`..`perf-06`) with explicit minimal-UX guardrail (advanced telemetry internal/details-only; default UI remains simple).
- 2026-02-26 A2 pass-4: corrected `go:S3776` count 12→13 (new hit `internal/config/env.go:72` CC=29, highest in codebase); verified OpenAPI spec drift (5 endpoints missing 500 docs); identified dead state fields (`state.ws`/`state.streamId`) and IIFE→module inconsistency in `results.js`/`skill.js`.
- Prefer behavior-preserving refactors + targeted regression tests over broad rewrites.
- Active backlog rows keep unresolved/external items only; this marathon closed all currently open rows (`Done` or `Cancelled`) and folded completion history into `Recently Closed IDs`.
- A1 fifth-pass analysis (2026-02-26): security/reliability findings (ClientIP spoofing chain, missing HSTS, UDP deadline syscall overhead, SDK timeout defaults, proxy port stripping) were implemented and verified in marathon wave.
- 2026-03-01 A2 pass-5: Sonar OPEN confirmed `0` (MCP live fetch); identified cancel-restart race in `startTest` catch (behavioral bug), frontend resilience gaps (loadServers timeout, localStorage.getItem guard, error body parsing), observability blind spots (stream failure reason logging, speed test request logging exclusion), `.env.example` missing 23 runtime env vars; browser baseline established at Safari 15.4 / Chrome 93 / Firefox 101.
- 2026-03-07 A0 closure wave: drained the active queue by landing the remaining runtime/web/docs/observability rows. `loadServers()` now times out and surfaces structured server errors, `loadSettings()` tolerates `localStorage.getItem` failures, failed stream completion can carry/stash explicit reasons for logs/status payloads, request logging now retains websocket + download visibility and logs only abnormal/slow upload requests while still skipping noisy `/ping`, and `.env.example` now documents the missing runtime/server/registry/TLS knobs. Focused verification green: `go test ./internal/api ./internal/stream ./test/unit/api ./test/unit/stream`, `bunx playwright test test/e2e/ui/basic.spec.js`.
- A1 sixth-pass analysis (2026-02-28): Deep dive into HTTP stream timeouts, SQLite performance, and client error handling. Identified a Slowloris vulnerability in HTTP test endpoints due to absolute timeouts. Found missing `PRAGMA synchronous=NORMAL` causing slow SQLite WAL inserts. Discovered uncaught `localStorage` exceptions breaking the web UI in incognito mode. Identified a client-side bug where IO errors leave orphaned streams running on the server. Added to active queue.
- A0 multi-pass critique (2026-03-01, skills-guided): added unique frontend/runtime/CI backlog rows (`20260301-web-07`, `20260301-a11y-02`, `20260301-ui-10`, `20260301-go-34`, `20260301-go-35`, `20260301-api-04`, `20260301-ws-02`, `20260301-ci-11`, `20260301-sec-07`) with evidence and focused checks; intentionally excluded overlaps with active A1 rows.

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
