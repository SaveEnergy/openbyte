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
| ID | Area | Agent | Status | Plan | Evidence | Check |
|---|---|---|---|---|---|---|
| — | — | A0 | Empty | Backlog pruned after full PDCA closeout; add only newly discovered work items here. | Completed rows archived in git history + last cycle notes below. | `go test -short ./...` |

### Analysis Snapshot (2026-02-05)
- Scope: backend runtime, client/web paths, storage/registry, CI/deploy workflows.
- Method: multi-pass review + targeted source validation (high-confidence issues only).
- Priority order: `stream-01` > `results-01` > `registry-01` > `api-02` > `web-02` > `api-03` > `stream-02` > `results-02` > `config-01` > `registry-02` > `web-01` > `api-04` > `metrics-01` > `compose-02` > `ci-01`.

### Last Completed Cycle
- Cycle: `2026-02-15 #24`
- Goal: close remaining reliability backlog (results + API concurrency/rollback edges)
- State: closed
- Completion evidence:
  - Results store retries transient BUSY/LOCKED writes and trims deterministically on equal timestamps.
  - Results `Get` error mapping distinguishes retryable (`503`) from internal (`500`) failures.
  - Speedtest upload read deadline aligns with configured max-duration contract.
  - Added rate-limiter cleanup concurrent stress test and rollback-cleanup failure branch regression.
  - Validation passed: targeted regressions + full `go test -short ./...`.

### Recent Decision Notes
- Used package-internal white-box tests for rollback/mapping branches hard to trigger from black-box HTTP tests.
- Added explicit semver tag-format guard in release deploy script for fail-fast behavior in reused/manual contexts.
- Applied rate-limit parity to registrar routes and browser results route.
- Tightened API mutation contract (explicit JSON content-type + unknown-field rejection).
- Preferred fail-fast CLI/config validation over silent fallback behavior.

### Archive Note
- Detailed completed queue rows and full event/decision history were intentionally pruned for readability.
- Canonical historical record remains in git history.

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

## Changelog Highlights

### v0.6.x
- Deployment/workflow hardening (config validation, checksum portability, network handling, force-recreate behavior).
- UI robustness and server-selection simplification.
- Server CLI flags for deploy-time config overrides (flags win when explicitly set).
- README branding improvements (theme-aware wordmark + flairs).

### v0.5.x
- MCP + SDK + `check` integration and structured diagnostics.
- OpenAPI publication and install/skill-page additions.

### v0.4.x
- Router migration to stdlib `ServeMux`.
- HTTP streaming parity and broad reliability/body-drain fixes.
- SQLite share-results flow and results page hardening.

### Pre-v0.4
- Core performance work: histogram, pooling, concurrency limits, CI/CD foundation.
