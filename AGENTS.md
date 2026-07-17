## Architecture Decisions

### Core Runtime

- Single `openbyte` server binary; server configuration is environment-only.
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
- Browser cancel paths propagate request contexts so aborts tear down transfer loops; cancelling always discards the incomplete run and returns to idle.
- Results store shutdown is explicit and idempotent enough for server lifecycle.
- Upload/download handlers enforce bounded concurrency, per-IP slots, max duration, and safe body deadlines.
- `MAX_CONCURRENT_TRANSFERS` is the explicit server-wide stream limit for each direction (default 200); `MAX_CONCURRENT_PER_IP` remains an independent per-direction client limit.

### Security & Validation

- Generic CORS is absent. `/api/v1/ping` alone sends wildcard CORS so eager IPv4/IPv6 discovery can read dedicated probe hosts; all other API routes are same-origin.
- CSP is strict (`script-src 'self'`, `worker-src 'self'`), with JS moved to external files only.
- The results POST handler enforces a 4096-byte limit, rejects unknown fields, and decodes exactly one JSON object in `internal/api/results_handler.go`.
- Config validation includes trusted CIDR parsing and strict positive limits.

### Frontend Behavior

- HTTP test mode uses `/api/v1/download`, `/api/v1/upload`, and `/api/v1/ping`; never TCP/UDP proxy mode.
- Self-hosted fonts, motion, loaded-latency measurement, and bufferbloat grading are intentional product features; simplification work must preserve them.
- English/German localization resolves once per page from the stored choice or browser-language Auto mode; changing it persists the choice and reloads. Navigation/share URLs never carry locale, static metadata stays server-authored English, and worker failures cross into the UI as catalog-key codes without prose.
- Network probe fetch paths drain non-OK and malformed JSON responses.
- Ping returns `client_ip` by default and adds `server_name` only for `?meta=1`; the UI infers IPv4/IPv6 from the canonical address while keeping all discovery probes eager.
- Server settings UI: no server selector; a single deployed server tests itself.
- UI render helpers guard missing DOM nodes to avoid runtime crashes in partial layouts.
- Header language/theme controls live in **`preferences.css`**; automatic language labels expose the resolved locale. The built-in **`--brand-primary`** stays mint while light-theme foregrounds use the separate accessible accent token; configured branding replaces the appropriate brand/accent tokens for each theme.
- Live and shared results put primary measurements before the loaded-latency advisory; they do not assign subjective speed or connection labels.
- Optional visual branding uses validated environment colors exposed through a
  generated same-origin `/branding.css` and a startup-loaded raster logo at
  `/branding/logo`; neutral and semantic status colors remain fixed for contrast.
  **`branding.js`** assigns the logo `src` only when `/branding.css` makes the
  logo visible, so unbranded deployments never request (and 404 on) the logo.
- Legal pages: `/privacy` serves the embedded, localized **`privacy.html`**
  describing actual data handling; the Impressum is never authored by openByte —
  a validated `IMPRESSUM_URL` makes `/impressum` redirect (302) to the
  operator's document and `/branding.css` unhide the footer link
  (`.footer-impressum`, hidden by default in `base.css`). Unconfigured
  deployments keep `/impressum` a 404 with no visible link and zero extra
  requests.
- Speed test: **`openbyte.js`** owns init/events/lifecycle/share/cancel; **`ui-results.js`** owns live result rendering; **`speedtest.js`** owns latency, the determinate progress model, and bridges UI state to the one-shot **`speedtest-worker.js`**; **`speedtest-adaptive.js`** chooses stream count/duration and reports ramp-window/measure progress; **`speedtest-http-{shared,download,upload}.js`** owns warm-up, progress, and transfer loops; **`network.js`** owns readiness and idle address probes (offline disables the start button); **`theme.js`** owns the manual light/dark override; **`history.js`** owns the localStorage recent-results list; **`stats-help.js`** injects the shared metric explanations on both `index.html` and `results.html`. Client IP discovery is a user-facing feature: same-origin and IPv4/IPv6 probes stay eager on page load, never deferred until **GO**; startup probes may finish during the first run but never overwrite completed results, and periodic probes mutate addresses only while idle. One exception: a `v4.`/`v6.` probe host that failed while the same-origin server was reachable is skipped for 24 h (localStorage negative cache) so unconfigured probe DNS does not log a console network error on every load. The same-origin bootstrap ping requests metadata to populate the configured server name; measurement, readiness, and address pings remain lean. Static serving derives its safe path set from assets embedded by **`web/embed.go`**; **`WEB_ROOT`** may override file contents but cannot expose additional paths.

### Storage

- Results store uses SQLite (`modernc.org/sqlite`, pure Go, no CGO), WAL mode via PRAGMA.
- Share IDs are short crypto-random base62.
- Retention and max-count cleanup are enforced with periodic pruning.
- Unique constraint detection uses typed sqlite error code with fallback message match.

### Agent & API Surface

- Agent integrations use the HTTP API / OpenAPI contract; the former `openbyte mcp` stdio server was removed pre-1.0.
- The former registry service/client/routes/config and web server selector were removed pre-1.0; use explicit URLs outside the app for multi-server comparisons.
- CLI speed-test clients and the Go SDK were removed during alpha; use the browser UI or HTTP API.
- The server-side TCP/UDP stream stack, `/api/v1/stream/*`, websocket stream API, `cmd/loadtest`, and direct test ports were removed pre-1.0.
- The unwired `internal/metrics` package, `pkg/types` RTT/NetworkInfo/CLI metric collectors, and the multi-server compose example were removed post-0.10.
- `api/openapi.yaml` is the canonical API contract; CI/release lint it, and the server has no duplicate browser API page. `/api/v1/version` was removed during alpha; bootstrap ping metadata supplies the server name.

### Build / CI / Deploy

- The official image and bundled Compose expose plain HTTP only on internal
  **8080** and persist at **`/app/data`**. The Dockerfile sets only that data
  path; Compose forwards explicit optional overrides while Go owns defaults.
  Direct TLS, HTTP/2 policy, and pprof remain binary/custom-deployment features.
- **Recovery**: Actions → `ci` → Run workflow on `main` if stuck; or `git fetch` via HTTPS if SSH fails.
- **`build-push` + `deploy`** on every `main` push or `main` workflow dispatch after `checks` (no path filtering—doc-only pushes still roll images). Dispatches from other refs run checks only. PR Playwright runs are gated by a plain `git diff` check inside `checks` (no third-party filter action).
- CI builds/pushes `edge` + `sha`; release publishes semver + `latest` images and Linux/macOS amd64/arm64 tarballs.
- **`release.yml` `deploy`**: same `vars`/secrets as CI; gate on **`needs.release.result == 'success'`** (not derived job booleans).
- Deploy: **checkout first**, then `scripts/deploy/deploy.sh` validates the host key, streams and checksums the bundle over one SSH connection, and runs `deploy_host.sh`; the previous openByte image is pinned locally for Compose-based rollback.
- `deploy_host.sh` requires Compose `up --wait-timeout`; deploy and rollback use
  60-second Compose-native health gates followed by image identity checks.
- Compose uses a published-image base; local source builds add `docker-compose.local.yaml`. The app healthcheck lives in the Dockerfile.
- Traefik deploy uses the external `traefik` network and generic HTTP/HTTPS routers only; workflows ensure network presence.
- **Race matrix**: `ci.yml` on `main`: `go test ./... -race -p 1`; `nightly.yml`: `go test -race ./...`.
- **Playwright**: `workers` = `2` on `GITHUB_ACTIONS`; optional `PLAYWRIGHT_WORKERS`; Playwright owns its local server and requires port 8080 to be free.
- **CI concurrency**: `cancel-in-progress` only for `pull_request`; `push`/`workflow_dispatch` queue on same `ref` (deploy not mid-aborted).
- **Nightly**: one full `go test -race ./...` gate. Performance and leak profiling are explicit local investigations, not unattended pass/fail theater.
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

- `go test ./cmd/openbyte`
- `go test ./test/unit/api ./test/unit/results`
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
make build && WEB_ROOT=./web ./bin/openbyte
```

- `WEB_ROOT=./web` serves static files from disk instead of the embedded copy, enabling live edits without rebuilding.
- Server listens on **:8080** (HTTP API + UI).
- Web UI: `http://localhost:8080`.

### Common commands

| Task                | Command                                                |
| ------------------- | ------------------------------------------------------ |
| Build               | `make build`                                           |
| Run server (dev)    | `make run` (or `WEB_ROOT=./web ./bin/openbyte`)        |
| Lint (Go)           | `make ci-lint`                                         |
| Lint (OpenAPI)      | `bun run lint:openapi`                                 |
| Go test suite       | `go test ./...`                                        |
| UI E2E (Playwright) | `make test-ui`                                         |
| Race detector       | `make test-race`                                       |
| Benchmarks          | `make perf-bench`                                      |

### Gotchas

- Playwright UI tests own a server on `127.0.0.1:8080`; the port must be free.
- No CGO required; SQLite uses `modernc.org/sqlite` (pure Go).
- No external databases or services needed to run locally.
- New web assets matching `web/embed.go` are served automatically; `WEB_ROOT` remains restricted to those embedded paths.
