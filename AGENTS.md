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
- **Advanced telemetry (policy)**: Supersedes backlog **`20260320-perf-03`** / deferred **`20260226-perf-02`**/**`04`**. Any future “Fast.com-style” depth stays **server/internal first** (config-gated endpoints, logs, pprof). **Default Web UI** remains the current simple speed test (no extra telemetry panels or competing primary metrics). **User-visible** detail views require **explicit opt-in** (dedicated env + UI affordance or URL mode)—never default-on. Implementation work stays out of scope until separately scoped.

### Reliability & Concurrency

- `sync.Once` on close/stop paths for idempotent shutdown (`Manager`, registry service, stream server).
- **`internal/stream` `Manager`**: `manager.go` (type + `New`/`Start`/`Stop`/`Set*`), `manager_streams.go` (create/start/complete/fail/query), `manager_cleanup.go` (retention loop + `releaseActiveStream*`), `manager_broadcast.go` (metrics channel fanout).
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
- HTTP speed test: **`speedtest-http.js`** re-exports **`speedtest-http-download.js`** / **`speedtest-http-upload.js`**; shared helpers in **`speedtest-http-shared.js`** (same public API for `speedtest.js`).

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

- Manual CI recovery: Actions → `ci` workflow → **Run workflow** on `main` if a push-triggered run is stuck (cancel the zombie run first). Same `checks` job as push; `build-push` still needs a real push or `workflow_dispatch` with **force build** where applicable.
- If `git fetch origin` fails (SSH agent): refresh `origin/main` with `git fetch https://github.com/SaveEnergy/openbyte.git +main:refs/remotes/origin/main`, or set `origin` to HTTPS and run `gh auth setup-git`.
- CI **`build-push`** + **`deploy`** run on **every** push to `main` (after `checks`); path `changes` only gates Playwright install on PRs—not Docker, so doc-only commits still publish images and can roll the test host.
- CI main builds/pushes `edge` + `sha`; release pipeline publishes semver + `latest`.
- **`release.yml` `deploy`** (test host) requires the same repo **`vars`** + secrets as CI **`deploy`**, and **`needs.release.result == 'success'`** (aligned with CI `deploy` ↔ `build-push`—do not gate deploy on a derived boolean job output; string coercion caused skipped deploys after green releases).
- Deploy path syncs compose files before remote execution to prevent server-side drift.
- Shared deploy shell: **`scripts/deploy/validate_env.sh`**, **`sync_compose.sh`**, **`deploy_remote.sh`** — invoked from **`ci.yml`** and **`release.yml`** (`DEPLOY_TAG` = `github.sha` vs semver tag without `v`); edit scripts once to update both pipelines.
- Deploy runs `docker compose pull` + `up -d --force-recreate`, then verifies expected image/container state.
- Traefik deploy uses external `traefik` network; workflows ensure network presence.
- Workflow gates require required deploy vars/secrets and fail fast on missing config.
- **Race detector matrix**: **`ci.yml`** (push / `workflow_dispatch` on **`main`**) runs **`go test ./... -race -short -p 1`** — **`-short`** skips tests that call **`skipIfShort`** (notably heavy **`test/e2e`** cases); **`-p 1`** runs packages serially to cap memory and contention on shared runners under race. **`nightly.yml`** runs **`go test -race ./...`** without **`-short`** (full race over all packages, including non-short e2e). Nightly also runs **`go test ./test/e2e`** separately (timeout budget) without race.
- **Playwright**: **`playwright.config.js`** sets **`workers`** to **`2`** when **`GITHUB_ACTIONS`** is set (typical GHA **2 vCPU**); otherwise Playwright default. Optional override: **`PLAYWRIGHT_WORKERS`**. **`trace: 'on-first-retry'`** and **`webServer.reuseExistingServer`** unchanged.
- **CI concurrency**: **`ci.yml`** uses **`cancel-in-progress: true`** only for **`pull_request`**; **`push`** ( **`main`** / tags ) and **`workflow_dispatch`** **do not** cancel an in-flight run — the next run **queues** on the same **`ref`**, so **`deploy`** is not aborted mid-job by a new push. Tradeoff: busy **`main`** can backlog sequential runs (prefer batching or squash if queue latency matters).
- **Nightly** (**`nightly.yml`**): runs **`make perf-bench`** each schedule (**`PERF_SMOKE`** no longer gates it). Optional **opt-out**: repo variable **`PERF_BENCH`** = **`false`**. **`make perf-leakcheck`** remains behind **`LEAK_PROFILE_SMOKE == 'true'`** (slow).
- **`make perf-bench`**: **`test/unit/metrics`**, **`test/unit/websocket`**, **`test/unit/stream`**, **`internal/api`** (**`respondJSON`**, **`validateMetricsPayload`**, **`normalizeHost`**), **`internal/jsonbody`** (**`DecodeSingleObject`**). Compare **main** vs branch: run **`go test <pkg> -run '^$' -bench . -benchmem -count=5 > /tmp/bench_a.txt`** on each tip, then **`benchstat /tmp/bench_a.txt /tmp/bench_b.txt`** (requires **`go install golang.org/x/perf/cmd/benchstat@latest`** or equivalent).
- Broader **CI / perf** backlog: **`20260320-ci-01`**..**`05`**, **`20260320-perf-01`**..**`03`** Done ( **`perf-03`** = telemetry **policy** in Architecture § Performance).

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

### Refactor analysis intake (2026-03-20 pass)

- **Executive snapshot**: No **critical** reliability/security defects surfaced in static LOC/churn scan; highest **maintainability** friction is **`cmd/client`** (large CLI + run stack) and **web** **`download.js`** / **`network.js`** (long single-file modules). **Test** weight is concentrated in **`test/unit/api/speedtest_test.go`** (~685 LOC).
- **Evidence (LOC, `wc`, top of tree)**:
  - **Go (runtime)**: `cmd/client` (`cli_flags.go`, `cli_usage.go`, `cli_validate.go`, `cli_servers.go`, `run.go`), `http_engine.go` (~370), `internal/stream/server.go` (~372), `test/unit/api/speedtest_test.go` (~685), `router_test.go` (~531).
  - **Web**: `web/download.js` (~432), `web/network.js` (~399), `speedtest-http-upload.js` (~312).
- **Assumptions**: `TODO`/`FIXME` grep empty in repo; **Sonar** QG **OK**; advanced telemetry remains **policy-only** (Architecture § Performance).
- **Action plan**: **Now** — none mandatory (behavior-preserving splits only when touching those areas). **Next** — **`download.js`** / **`network.js`** extract platform table + fetch helpers; optional **`internal/stream/server.go`** TCP seam. **Later** — **`pkg/client`** SDK file split (already noted in Open/Deferred).

### Live Queue (active only)

| ID | Area | Agent | Status | Plan | Evidence | Check |
| --- | --- | --- | --- | --- | --- | --- |
| `20260320-refactor-15` | web | - | Planned | Reduce **`download.js`** / **`network.js`** coupling: extract platform constants + pure helpers; keep `openbyte.js` orchestration boundary. | **~432** / **~399** LOC. | `npx prettier --check web/*.js`; `bunx playwright test test/e2e/ui/basic.spec.js` |
| `20260320-refactor-16` | stream | - | Planned | Optional: split **`internal/stream/server.go`** along TCP accept/read/write vs lifecycle helpers (same `Server` API). | **~372** LOC; **`server_udp.go`** already split. | `go test ./internal/stream/... ./test/unit/stream/...` |

### Check Hold (manual/external)

| ID | Area | Agent | Status | Plan | Evidence | Check |
| --- | --- | --- | --- | --- | --- | --- |
| _none_ | - | - | - | No pending manual/external checks. | Last wave completed with local verification. | N/A |

### Sonar Snapshot (latest recheck)

- Strict OPEN filter parity maintained with Cloud:
  - Query: `projects=[SaveEnergy_openbyte]`, `issueStatuses=[OPEN]`, `ps=500`
  - **2026-03-20 code fixes** (prior **27** OPEN wave): **`[[`** in **`scripts/deploy/{validate_env,sync_compose,deploy_remote}.sh`**; **`TestDecodeSingleObject*`** in **`internal/jsonbody/decode_test.go`**; **`go:S1192`** constants; **`resolvePlaywrightWorkers()`** in **`playwright.config.js`**; **`execContexter`** in **`internal/results/store_migrate.go`**; **`init?.signal`** / **`signal?.`** in **`test/e2e/ui/basic.spec.js`** (**`javascript:S6582`**); success toast **`<output>`** in **`web/index.html`** + assertion.
  - **Post-analysis (MCP)**: **Quality gate `OK`** — including **`new_security_hotspots_reviewed`** = **`100%`**; **`javascript:S6582`** on **`basic.spec.js`** addressed with **`?.`** (re-verify OPEN count after next Cloud analysis).
  - Historical: **`27`** OPEN (**2026-03-20** pre-fix); **`0`** OPEN (**2026-03-01**); **`23`** OPEN (**2026-02-26**)
- Sonar MCP exposes issue search + metrics + QG status; **hotspot review** is also available in **Sonar UI**.

### Recently Closed IDs

- Most historical IDs intentionally pruned for readability; canonical record remains in git history.
- Recent close: `20260319-refactor-01`..`13` (refactor wave); `20260320-refactor-14` (**`cmd/client`** **`cli.go`** → **`cli_{flags,usage,validate,servers}.go`**).
- Latest completed wave (moved `Check -> Done -> removed`):
  - `20260320-refactor-14` (**`cmd/client`**: behavior-preserving split of former **`cli.go`** into **`cli_flags.go`**, **`cli_usage.go`**, **`cli_validate.go`**, **`cli_servers.go`**; `go test ./cmd/client/...` green)
  - `20260320-ci-01`, `20260320-ci-02` (CI **`govulncheck`**; **Redocly** pinned in **`package.json`**, **`bun run lint:openapi`** in **`ci.yml`**/**`release.yml`**, single **`bun install`** before OpenAPI + Playwright)
  - `20260320-ci-03` (documented **CI** vs **nightly** race matrix: **`-short`** + **`-p 1`** on **`main`**; full **`go test -race ./...`** nightly; workflow comments)
  - `20260320-ci-04` (**`playwright.config.js`**: **`workers`** = **`2`** on **`GITHUB_ACTIONS`**; **`PLAYWRIGHT_WORKERS`** override)
  - `20260320-ci-05` (**`ci.yml`**: **`cancel-in-progress`** only for **`pull_request`**; **`push`**/**`workflow_dispatch`** queue on **`ref`** — no mid-**`deploy`** cancel)
  - `20260320-perf-01` (**`nightly.yml`**: **`make perf-bench`** default; **`PERF_BENCH`** = **`false`** opt-out; **`LEAK_PROFILE_SMOKE`** unchanged)
  - `20260320-perf-02` (**`Makefile`** **`perf-bench`**: **`internal/api`** + **`internal/jsonbody`** benches; **benchstat** doc in **AGENTS**)
  - `20260320-perf-03` (Architecture § Performance **advanced telemetry** policy: internal-first, minimal default UI, explicit opt-in for any user-visible details)
  - `20260319-refactor-01`, `20260319-refactor-02`, `20260319-refactor-03`, `20260319-refactor-04`, `20260319-refactor-05`, `20260319-refactor-06`, `20260319-refactor-07`, `20260319-refactor-08`, `20260319-refactor-09`, `20260319-refactor-10`, `20260319-refactor-11`, `20260319-refactor-12`, `20260319-refactor-13`
  - `20260228-sec-06`, `20260228-go-32`, `20260228-ui-09`, `20260228-go-33`, `20260301-web-07`, `20260301-a11y-02`, `20260301-ui-10`, `20260301-go-34`, `20260301-go-35`, `20260301-api-04`, `20260301-ws-02`, `20260301-ci-11`, `20260301-sec-07`, `20260301-web-06`, `20260301-web-08`, `20260301-ops-01`, `20260301-doc-02`
  - `20260217-web-02`, `20260217-go-02`, `20260217-go-03`, `20260217-go-04`, `20260217-go-05`, `20260217-go-06`, `20260217-go-07`, `20260217-go-08`, `20260217-go-09`
  - `20260217-test-02`, `20260217-test-03`, `20260217-test-04`, `20260217-test-05`, `20260217-test-06`, `20260217-test-07`
  - `20260217-sec-01`, `20260218-go-12`, `20260218-go-13`, `20260219-ui-01`, `20260219-ui-02`, `20260219-web-02`, `20260219-web-05`, `20260219-web-06`, `20260219-ui-03`, `20260219-cli-03`, `20260219-go-16`, `20260219-cli-01`, `20260219-cli-02`, `20260219-ui-04`, `20260219-ui-05`, `20260219-go-15`, `20260217-test-09`, `20260217-test-10`, `20260219-go-17`, `20260219-go-18`, `20260219-go-19`, `20260219-ci-01`, `20260219-doc-01`, `20260219-ui-06`, `20260219-ui-07`, `20260219-go-20`, `20260219-go-21`, `20260220-sec-01`, `20260220-api-01`, `20260219-go-22`, `20260220-web-01`, `20260220-meta-01`, `20260219-sdk-01`, `20260219-reg-01`, `20260219-test-13`, `20260219-test-11`, `20260219-test-12`, `20260226-sec-02`, `20260226-sonar-01`, `20260226-sonar-02`, `20260226-ci-10`, `20260226-go-24`, `20260226-go-25`, `20260226-go-26`, `20260226-sonar-03`, `20260226-api-02`, `20260226-web-03`, `20260226-go-04`, `20260226-web-04`, `20260226-sonar-04`, `20260226-sonar-05`, `20260226-sonar-06`, `20260226-sonar-07`, `20260226-sonar-08`, `20260226-sonar-09`, `20260226-perf-03`, `20260226-perf-05`, `20260226-perf-06`, `20260226-sec-03`, `20260226-sec-04`, `20260226-go-27`, `20260226-go-28`, `20260226-go-29`, `20260226-api-03`, `20260226-web-05`
- Marathon deferred/cancelled by design-risk guardrail: `20260226-perf-02`, `20260226-perf-04`.

### Recent Decision Notes

- 2026-03-20: **`20260320-refactor-14` Done** — **`cmd/client`**: split former **`cli.go`** into **`cli_flags.go`**, **`cli_usage.go`**, **`cli_validate.go`**, **`cli_servers.go`** (same **`package client`**); **`run.go`** unchanged; `go test ./cmd/client/...` green.
- 2026-03-20: **Refactor analysis (pass)** — **Refactor analysis intake** + Live Queue **`20260320-refactor-15`**..**`16`** ( **`web`** **`download.js`**/**`network.js`**, **`internal/stream/server.go`** split optional); evidence **`wc`**, empty **`TODO`** grep, **Sonar** QG **OK**.
- 2026-03-20: **Sonar follow-up** — **QG `OK`** on Cloud (**hotspots** **`100%`**); **`javascript:S6582`** in **`basic.spec.js`** resolved with **`init?.signal`** / **`signal?.`** (not `&&`); **Sonar Snapshot** updated.
- 2026-03-20: **Sonar OPEN fixes landed** — **`shelldre`**, **`go:S100`**, **`go:S1192`**, **`javascript:S3358`**, **`godre:S8196`**, **`Web:S6819`** + first **`S6582`** pass (see **Sonar Snapshot**).
- 2026-03-20: **`20260320-perf-03` Done** — **Advanced telemetry** guardrail documented under Architecture § Performance (internal/server-first, default UI unchanged, opt-in only for client-visible detail); defers implementation; ties to marathon **`20260226-perf-02`**/**`04`** intent without reviving marathons.
- 2026-03-20: **`20260320-perf-02` Done** — **`internal/api/handlers_bench_test.go`**, **`internal/jsonbody/decode_bench_test.go`**; **`Makefile`** **`perf-bench`** extended; **benchstat** compare documented in **AGENTS** (manual).
- 2026-03-20: **`20260320-perf-01` Done** — **`nightly.yml`**: **`make perf-bench`** runs unless repo **`vars.PERF_BENCH`** is **`false`** (replaces **`PERF_SMOKE`** gate); **`perf-leakcheck`** still **`vars.LEAK_PROFILE_SMOKE == 'true'`**.
- 2026-03-20: **`20260320-ci-05` Done** — **`ci.yml`** **`cancel-in-progress: ${{ github.event_name == 'pull_request' }}`** — PRs still cancel superseded runs; **`main`**/tags/dispatch **queue** (same concurrency **group** + **ref**) so an in-flight **`deploy`** is not aborted by a new push; tradeoff: **`main`** backlog under burst pushes.
- 2026-03-20: **`20260320-ci-04` Done** — **`playwright.config.js`**: explicit **`workers`** (**`2`** when **`GITHUB_ACTIONS`**); optional **`PLAYWRIGHT_WORKERS`**; **`trace`** / **`reuseExistingServer`** unchanged.
- 2026-03-20: **`20260320-ci-03` Done** — Architecture + **`ci.yml`**/**`nightly.yml`** comments document why **`main`** race uses **`-short -p 1`** and nightly uses full **`go test -race ./...`** (no redundant **`-short`** on nightly).
- 2026-03-20: **`20260320-ci-01`**/**`02` Done** — **`checks`** runs **`go run golang.org/x/vuln/cmd/govulncheck@latest ./...`**; **`@redocly/cli@2.18.1`** in **`package.json`** with **`lint:openapi`** script; CI/release use **`bun install --no-save`** once then **`bun run lint:openapi`** (no cold **`npx`**); **`Makefile`** **`lint-openapi`** for local parity.
- 2026-03-20: **CI/perf backlog intake** — Added Live Queue **`20260320-ci-01`**..**`05`**, **`20260320-perf-01`**..**`03`**: evidence from `ci.yml`, `nightly.yml`, `Makefile`, `playwright.config.js`, AGENTS deferred perf rows; prioritize **`ci-01`** (govulncheck automation) and **`ci-02`** (OpenAPI lint cost) for security + minutes.
- 2026-03-19: **`20260319-refactor-13` Done** — `internal/stream`: split `manager.go` into `manager_streams.go`, `manager_cleanup.go`, `manager_broadcast.go` + slim `manager.go`; `go test ./internal/stream/... ./test/unit/stream/...` green.
- 2026-03-19: **`20260319-refactor-12` Done** — `internal/api`: replaced `router_middleware.go` with `router_middleware_ratelimit.go`, `router_middleware_cors.go`, `router_middleware_logging.go`, `router_middleware_security.go` (Deadline + security headers); `router.go` wrap order unchanged; `go test ./internal/api/... ./test/unit/api/...` green.
- 2026-03-19: **`20260319-refactor-11` Done** — `web/speedtest-http.js` split: `speedtest-http-shared.js`, `speedtest-http-download.js`, `speedtest-http-upload.js`, barrel `speedtest-http.js`; Prettier clean; `speedtest.js` import unchanged.
- 2026-03-19: **`20260319-refactor-10` Done** — `internal/results`: `store.go` (types, `New`, `Close`), `store_migrate.go` (schema + PRAGMA pool), `store_id.go` (`generateID`), `store_crud.go` (`Save`/`Get`, busy/unique helpers), `store_cleanup.go` (retention loop); `go test ./internal/results/... ./test/unit/results/...` green.
- 2026-03-19: **`20260319-refactor-09` Done** — CI + `release.yml` **`deploy`** jobs call shared **`scripts/deploy/*.sh`** (validate, sync compose, remote pull/up); `DEPLOY_TAG` from `github.sha` (CI) or `GITHUB_REF_NAME` semver strip (release); `bash -n` on scripts locally.
- 2026-03-19: **`20260319-refactor-08` Done** — `internal/config`: replaced single `env.go` with `env_helpers.go` (shared parsers + `EnvDebug`), `env_core.go` (ports/bind/meta/capacity), `env_extended.go` (runtime, limits/network, storage, registry, TLS); `LoadFromEnv` unchanged in `config.go`; `go test ./test/unit/config/...` + `./... -short` green.
- 2026-03-19: **`20260319-refactor-07` Done** — `cmd/server`: `flags.go` (CLI + overrides), `runtime.go` (wiring, HTTP, shutdown, `broadcastMetrics`), thin `main.go` (`Run`); `go test ./cmd/server/...` + `go test ./... -short` green.
- 2026-03-19 (deep analysis): Added Live Queue `20260319-refactor-07`..`13` — prioritized by **maintainability × testability** (evidence: LOC clusters, duplicate workflows, existing unit/E2E hooks); **no** overlap with completed `01`..`06` scope; staged behavior-preserving splits per Engineering Guardrails.
- 2026-03-19: Landed `20260319-refactor-01`..`06`: package `internal/jsonbody`; websocket files `origin.go`/`broadcast.go`/`limits.go`/`ping.go`/`lifecycle.go`/`message_types.go` + slim `server.go`; `speedtest_{download,upload,deadline}.go`; `handlers_meta.go`/`handlers_stream.go`; SDK `client_http.go`; `refactor-06` = AGENTS frontend ownership note only (no JS moves).
- 2026-03-19: Post-refactor gates green: `gofmt` on `internal/api/speedtest_download.go`, `make ci-lint`, `make ci-test`, Redocly lint + `TestOpenAPIRouteContract`, `go mod tidy` clean, `go test ./... -race -short -p 1`, full `bunx playwright test`.
- 2026-03-19: Security hygiene: `go 1.26.1`, indirect `github.com/buger/jsonparser v1.1.2` (Dependabot alert on transitive `mcp-go` → `jsonschema` → `go-ordered-map`), `docker/Dockerfile` builder `golang:1.26.1-alpine`; `govulncheck ./...` clean for reachable symbols.
- 2026-03-19: Dependency refresh: `golang.org/x/term v0.41.0`, `modernc.org/sqlite v1.47.0` (plus transitive `x/sys`, `modernc.org/libc`); supersedes open Dependabot PRs for those direct deps.
- 2026-03-19: CI: `workflow_dispatch` on `main` now runs **race** tests (same as `push`); push-triggered run `23316903684` was cancelled after stuck `checks`; dispatch run `23317250010` completed green.
- 2026-03-19: **v0.8.0**: root **CHANGELOG.md** added; tag follows **v*.*.*** → `release.yml` (assets, GH Release, images).
- 2026-03-19: **v0.8.0** shipped: `release.yml` run **23317670813** green; [GitHub Release](https://github.com/SaveEnergy/openbyte/releases/tag/v0.8.0) + multi-arch binaries + `checksums.txt`.
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

- Rich telemetry / “details” UI beyond the **Architecture § Performance** guardrail (policy done; product build unscheduled).
- Public hosted test fleet (infra/cost decision).
- Additional SDKs from OpenAPI (TypeScript/Python).
- Packaging/distribution polish (Homebrew/apt repos).
