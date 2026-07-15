## Architecture Decisions

### Core Runtime

- Single `openbyte` binary with `server` / `check` subcommands.
- Routing uses stdlib `net/http.ServeMux` (`METHOD /path/{param}` + `r.PathValue`).
- Web assets are embedded (`//go:embed`) with optional `WEB_ROOT` override for development.
- Runtime is HTTP-only: no MCP server, registry, server selector, downloads page, TCP/UDP data ports, `/api/v1/stream/*`, or websocket metrics feed.

### Performance

- HTTP/UI speed logic uses warm-up gating and EWMA smoothing for stable live display.
- Browser tests use adaptive HTTP stream ramping in a module Web Worker, then measure with the selected stream count.
- Allowlisted text assets are gzipped lazily and cached per file version; fonts and byte-range responses stay identity encoded.
- **Advanced telemetry (policy)**: Future depth stays **server/internal first** (config-gated, logs, pprof). **Default Web UI** stays the simple speed test. **User-visible** detail requires **explicit opt-in** (env + UI or URL mode)—never default-on.

### Reliability & Concurrency

- Request bodies are drained only within strict byte/time bounds; unexpected bodyless-route bodies and incomplete drains are aborted without sacrificing the HTTP/2 connection.
- SDK/browser cancel paths propagate request contexts so aborts tear down transfer loops.
- Results store shutdown is explicit and idempotent enough for server lifecycle.
- Upload/download handlers enforce bounded concurrency, per-IP slots, max duration, and safe body deadlines.

### Security & Validation

- CORS wildcard matching enforces safe dot-boundary behavior.
- CSP is strict (`script-src 'self'`, `worker-src 'self'`), with JS moved to external files only.
- The results POST handler enforces a 4096-byte limit, rejects unknown fields, and decodes exactly one JSON object in `internal/api/results_handler.go`.
- Config validation includes trusted CIDR parsing and strict positive limits.

### Frontend Behavior

- HTTP test mode uses `/api/v1/download`, `/api/v1/upload`, and `/api/v1/ping`; never TCP/UDP proxy mode.
- Network and version probe fetch paths drain non-OK and malformed JSON responses.
- Server settings UI: no server selector; a single deployed server tests itself.
- UI render helpers guard missing DOM nodes to avoid runtime crashes in partial layouts.
- Speed test: **`speedtest-orchestrator.js`** owns lifecycle/share; **`speedtest.js`** owns latency and bridges UI state to the one-shot **`speedtest-worker.js`**; **`speedtest-adaptive.js`** chooses stream count/duration; **`speedtest-http-{shared,download,upload}.js`** owns warm-up, progress, and transfer loops. Thin **`openbyte.js`** owns init/events; **`network.js`** owns readiness and address probes. Client IP discovery is a user-facing feature: same-origin and IPv4/IPv6 probes stay eager on page load, never deferred until **GO**. API docs are **`api.html`** + **`api.css`**. Static serving derives its safe path set from assets embedded by **`web/embed.go`**; **`WEB_ROOT`** may override file contents but cannot expose additional paths.

### Storage

- Results store uses SQLite (`modernc.org/sqlite`, pure Go, no CGO), WAL mode via PRAGMA.
- Share IDs are short crypto-random base62.
- Retention and max-count cleanup are enforced with periodic pruning.
- Unique constraint detection uses typed sqlite error code with fallback message match.

### Agent & API Surface

- Agent integrations use the HTTP API / OpenAPI contract and Go SDK; the former `openbyte mcp` stdio server was removed pre-1.0.
- The former registry service/client/routes/config and web server selector were removed pre-1.0; use explicit URLs outside the app for multi-server comparisons.
- The full CLI speed-test client was removed during alpha; use the browser UI, HTTP API, Go SDK, or `openbyte check`.
- The server-side TCP/UDP stream stack, `/api/v1/stream/*`, websocket stream API, `cmd/loadtest`, and direct test ports were removed pre-1.0.
- The unwired `internal/metrics` package, `pkg/types` RTT/NetworkInfo/CLI metric collectors, and the multi-server compose example were removed post-0.10.
- Go SDK (`pkg/client`): `Check`, `SpeedTest`, `Diagnose`, `Healthy`; implementation split across `client.go` + `client_{check,speedtest,diagnose,health,measure}.go` (same exported API). The SDK uses stateless request/body handling from `internal/httptransfer`.
- OpenAPI spec lives at `api/openapi.yaml`; CI/release lint it.
- `openbyte check --json` supports schema versioning and structured error contracts.

### Build / CI / Deploy

- Docker exposes only **8080** (HTTP API + UI).
- **Recovery**: Actions → `ci` → Run workflow on `main` if stuck; or `git fetch` via HTTPS if SSH fails.
- **`build-push` + `deploy`** on every `main` push after `checks` (no path filtering—doc-only pushes still roll images). PR Playwright runs are gated by a plain `git diff` check inside `checks` (no third-party filter action).
- CI builds/pushes `edge` + `sha`; release publishes semver + `latest`.
- **`release.yml` `deploy`**: same `vars`/secrets as CI; gate on **`needs.release.result == 'success'`** (not derived job booleans).
- Deploy: **checkout first**, then `scripts/deploy/deploy.sh` validates the host key, streams and checksums the bundle over one SSH connection, and runs `deploy_host.sh`; the previous openByte image is pinned locally for Compose-based rollback.
- Traefik deploy uses external `traefik` network; workflows ensure network presence.
- **Race matrix**: `ci.yml` on `main`: `go test ./... -race -short -p 1`; `nightly.yml`: full `go test -race ./...` (including E2E once).
- **Playwright**: `workers` = `2` on `GITHUB_ACTIONS`; optional `PLAYWRIGHT_WORKERS`; trace/reuse unchanged.
- **CI concurrency**: `cancel-in-progress` only for `pull_request`; `push`/`workflow_dispatch` queue on same `ref` (deploy not mid-aborted).
- **Nightly**: `make perf-bench` each run unless `PERF_BENCH=false`; `perf-leakcheck` still behind `LEAK_PROFILE_SMOKE`.
- **`make perf-bench`**: runs the curated transfer/gzip/JSON/SQLite suite from **`test/perf/bench_packages.txt`**; explicit experiments save output and use `benchstat` manually. See **`test/perf/README.md`**.

## Engineering Guardrails

- Keep behavior changes minimal and explicit; avoid orthogonal refactors in reliability passes.
- Prefer fixing root-cause over masking symptoms.
- Add regression tests for bug fixes; strengthen existing tests instead of broad rewrites.
- Keep docs aligned with actual workflow/runtime behavior after each operational change.

## Dynamic Backlog

- Re-run throughput gates on target 25G hardware: direct TLS h2 vs `HTTP2_ENABLED=false`, Traefik `openbyte-h1@file` vs `openbyte-h2@file`, `MODE=download-shards`, `MODE=upload-shards`, and h2 request-stream upload.
- If target hardware shows a clear sharding win (>=25% median), implement production multi-worker sharding; otherwise keep it as harness-only.
- Rich telemetry UI remains deferred by policy unless explicitly opt-in.

## Verification baseline

- `go test ./cmd/check ./cmd/server ./cmd/openbyte`
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

- **Go 1.26.5+** (`go.mod` baseline; `go` auto-downloads the toolchain if the VM preinstall is older).
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
| Short Go suite      | `go test ./... -short`                                 |
| E2E tests (Go)      | `make test-e2e`                                        |
| UI E2E (Playwright) | `make test-ui`                                         |
| Race detector       | `make test-race`                                       |
| Benchmarks          | `make perf-bench`                                      |

### Gotchas

- Playwright UI tests start a server on `127.0.0.1:8080`, or reuse one already running there.
- No CGO required; SQLite uses `modernc.org/sqlite` (pure Go).
- No external databases or services needed to run locally.
- New web assets matching `web/embed.go` are served automatically; `WEB_ROOT` remains restricted to those embedded paths.
- `openbyte check` needs a full URL scheme: `./bin/openbyte check http://localhost:8080`, not just `localhost`.
