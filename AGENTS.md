## Architecture Decisions

### Core Runtime

- Single `openbyte` binary with `server` / `client` / `check` subcommands.
- Routing uses stdlib `net/http.ServeMux` (`METHOD /path/{param}` + `r.PathValue`).
- Web assets are embedded (`//go:embed`) with optional `WEB_ROOT` override for development.
- Runtime is HTTP-only: no MCP server, registry, server selector, downloads page, TCP/UDP data ports, `/api/v1/stream/*`, or websocket metrics feed.

### Performance

- Fixed-bucket latency histogram (1ms buckets, 2s window) replaces sort-heavy percentile computation.
- HTTP/UI speed logic uses warm-up gating and EWMA smoothing for stable live display.
- Browser tests use adaptive HTTP stream ramping in a module Web Worker, then measure with the selected stream count.
- **Advanced telemetry (policy)**: Future depth stays **server/internal first** (config-gated, logs, pprof). **Default Web UI** stays the simple speed test. **User-visible** detail requires **explicit opt-in** (env + UI or URL mode)—never default-on.

### Reliability & Concurrency

- Request/response bodies are drained on error paths to preserve HTTP/2 connection reuse.
- HTTP client/browser cancel paths propagate request contexts so aborts tear down transfer loops.
- Results store shutdown is explicit and idempotent enough for server lifecycle.
- Upload/download handlers enforce bounded concurrency, per-IP slots, max duration, and safe body deadlines.

### Security & Validation

- CORS wildcard matching enforces safe dot-boundary behavior.
- CSP is strict (`script-src 'self'`, `worker-src 'self'`), with JS moved to external files only.
- JSON API handlers enforce size limits and single-object decoding for POST payloads (`internal/jsonbody.DecodeSingleObject` shared by API + results).
- Config validation includes trusted CIDR parsing and strict positive limits.

### Frontend Behavior

- HTTP test mode uses `/download`, `/upload`, `/ping`; never TCP/UDP proxy mode.
- Network probe and health-check fetch paths drain non-OK and malformed JSON responses.
- Server settings UI: no server selector; a single deployed server tests itself.
- UI render helpers guard missing DOM nodes to avoid runtime crashes in partial layouts.
- Speed test: **`speedtest-orchestrator.js`** (lifecycle + share) + thin **`openbyte.js`** init; **`speedtest.js`** bridges UI state to **`speedtest-worker.js`**; **`speedtest-adaptive.js`** chooses stream count/duration; **`speedtest-http.js`** barrels **`speedtest-http-{shared,download,upload}.js`** (shared warmup/progress via **`applyHttpMeasureTick`** in **`speedtest-http-shared.js`**); API docs page is **`api.html`** + **`api.css`**; network **`network-{helpers,health,probes}.js`** + **`network.js`**. Any new top-level **`web/*.js`** (or HTML/CSS) must be added to **`internal/api/router_static.go`** allowlist or the server returns **404**.

### Storage

- Results store uses SQLite (`modernc.org/sqlite`, pure Go, no CGO), WAL mode via PRAGMA.
- Share IDs are short crypto-random base62.
- Retention and max-count cleanup are enforced with periodic pruning.
- Unique constraint detection uses typed sqlite error code with fallback message match.

### Agent & API Surface

- Agent integrations use the HTTP API / OpenAPI contract and Go SDK; the former `openbyte mcp` stdio server was removed pre-1.0.
- The former registry service/client/routes/config and web server selector were removed pre-1.0; use explicit URLs outside the app for multi-server comparisons.
- The CLI client is HTTP-only; TCP/UDP CLI testing, bidirectional CLI mode, and installer/download web page were removed pre-1.0.
- The server-side TCP/UDP stream stack, `/api/v1/stream/*`, websocket stream API, `cmd/loadtest`, and direct test ports were removed pre-1.0.
- Go SDK (`pkg/client`): `Check`, `SpeedTest`, `Diagnose`, `Healthy`; implementation split across `client.go` + `client_{check,speedtest,diagnose,health,latency,download,upload}.go` (same exported API).
- OpenAPI spec lives at `api/openapi.yaml`; CI/release lint it.
- JSON output supports schema versioning and structured error contracts.

### Build / CI / Deploy

- Docker exposes only **8080** (HTTP API + UI).
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
- **`make perf-bench`**: runs **`scripts/perf/run_benchmarks.sh`** (package list **`test/perf/bench_packages.txt`**) to stdout; **`make perf-record`** → **`build/perf/bench.txt`** for **`make perf-compare`** (**`benchstat`** on PATH, else **`go run golang.org/x/perf/cmd/benchstat@latest`**). See **`test/perf/README.md`**.

## Engineering Guardrails

- Keep behavior changes minimal and explicit; avoid orthogonal refactors in reliability passes.
- Prefer fixing root-cause over masking symptoms.
- Add regression tests for bug fixes; strengthen existing tests instead of broad rewrites.
- Keep docs aligned with actual workflow/runtime behavior after each operational change.

## Dynamic Backlog

- Live queue is currently empty; replenish with LOC/import-coupling scan plus Sonar OPEN count after next Cloud analysis.
- Rich telemetry UI remains deferred by policy unless explicitly opt-in.

## Verification baseline

- `go test ./cmd/check ./cmd/server ./cmd/client`
- `go test ./test/unit/api ./test/unit/client ./test/unit/results`
- `go test ./internal/results`
- `bun run lint:openapi`
- UI E2E: `PLAYWRIGHT_WORKERS=1 bunx playwright test test/e2e/ui/basic.spec.js`

### Test layout

- Prefer `test/`; legacy white-box tests may stay under `cmd/` / `internal/` for package-private access.

## Open / Deferred

- Rich telemetry UI beyond Architecture § Performance policy.
- Public hosted test fleet (infra/cost).
- Additional SDKs from OpenAPI (TypeScript/Python).
- Packaging polish (Homebrew/apt).

## Cursor Cloud specific instructions

### Prerequisites

- **Go 1.26.1** (pre-installed in the VM).
- **Node.js 22** + **bun** (install via `npm install -g bun` if missing).
- JS dev deps: `bun install` in repo root (Playwright + Redocly CLI).
- Playwright browser: `bunx playwright install chromium --with-deps`.

### Running the server (development)

```bash
make build && WEB_ROOT=./web ./bin/openbyte server
```

- `WEB_ROOT=./web` serves static files from disk instead of the embedded copy, enabling live edits without rebuilding.
- Server listens on **:8080** (HTTP API + UI).
- Web UI: `http://localhost:8080`.

### Common commands

| Task                | Command                                                |
| ------------------- | ------------------------------------------------------ |
| Build               | `make build`                                           |
| Run server (dev)    | `make run` (or `WEB_ROOT=./web ./bin/openbyte server`) |
| Lint (Go)           | `make ci-lint`                                         |
| Lint (OpenAPI)      | `bun run lint:openapi`                                 |
| Unit tests          | `go test ./... -short`                                 |
| E2E tests (Go)      | `go test ./test/e2e -short`                            |
| UI E2E (Playwright) | `make test-ui` (requires running server)               |
| Race detector       | `make test-race`                                       |
| Benchmarks          | `make perf-bench`                                      |

### Gotchas

- The server must be running before Playwright UI tests (`make test-ui`).
- No CGO required; SQLite uses `modernc.org/sqlite` (pure Go).
- No external databases or services needed to run locally.
- New `web/*.js` or `web/*.css` files must be added to `internal/api/router_static.go` allowlist or the server returns 404.
- CLI client needs full URL scheme: `./bin/openbyte client http://localhost:8080`, not just `localhost`.
