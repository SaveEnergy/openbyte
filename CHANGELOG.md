# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **`make perf-compare`**: runs **`benchstat`** from **`PATH`** when present, otherwise **`go run golang.org/x/perf/cmd/benchstat@latest`** (no separate install required for autoresearch agents).
- **internal/registry**: split **`handler.go`** into **`handler_list.go`** (list/get) + **`handler_mutations.go`** (register/update/deregister/health + `respondRegistryError`); split **`client.go`** into **`client_http.go`** (register/heartbeat/deregister + `drainAndClose`), **`client_loop.go`** (timer/backoff/jitter), **`client_info.go`** (`buildServerInfo`); **`20260322-refactor-05`** Done (`go test ./internal/registry/... ./test/unit/registry/...`).
- **cmd/client**: split **`engine.go`** / removed **`engine_direction.go`** → **`engine_dial.go`** (TCP/UDP dial, capture NAT info), **`engine_readwrite.go`** (download/upload + shared read/write loops), **`engine_bidirectional.go`**, **`engine_latency.go`** (latency/jitter stats, `isTimeoutError`); split **`formatter.go`** → **`formatter_classify.go`**, **`formatter_json.go`**, **`formatter_plain.go`**, **`formatter_interactive.go`**, **`formatter_ndjson.go`**, **`formatter_helpers.go`**; **`20260322-refactor-04`** Done (`go test ./cmd/client/...`).
- **test/unit/stream**: split **`manager_test.go`** into **`manager_test_common_test.go`**, **`manager_lifecycle_test.go`**, **`manager_limits_test.go`**, **`manager_broadcast_metrics_test.go`**, **`manager_terminal_test.go`**; **`20260322-refactor-03`** Done (`go test ./test/unit/stream/...`).
- **test/unit/results**: split **`store_test.go`** into **`store_test_common_test.go`**, **`store_crud_test.go`**, **`store_retention_test.go`**, **`store_busy_test.go`**, **`store_handler_routes_test.go`**; **`20260322-refactor-02`** Done (`go test ./test/unit/results/...`).
- **test/unit/api**: split **`router_test.go`** into **`router_test_common_test.go`** (constants + registry test stub), **`router_middleware_stream_test.go`**, **`router_static_cache_ratelimit_test.go`**, **`router_results_api_routes_test.go`**, **`router_static_allowlist_test.go`**; **`20260322-refactor-01`** Done (`go test ./test/unit/api/...`).
- **AGENTS.md**: compacted — merged refactor intake narratives into short backlog notes; single slimmer Live Queue table; trimmed Sonar/closed-ID/decision-note walls; **~280 → ~154 lines** (detail remains in CHANGELOG / git history).

### Added

- **Perf autoresearch**: **`make autoresearch-preflight`** + **`scripts/perf/autoresearch_preflight.sh`** (exit code + **`AUTORESEARCH_*`** stdout); tracked **`test/perf/AUTORESEARCH_CURSOR_COMMAND.md`** as the canonical Cursor **`/autoresearch`** body; **`PROMPT_AUTORESEARCH.md`** / **`test/perf/README.md`** / **`AGENTS.md`** aligned (resume-on-branch, autonomous default, **`make perf-compare`** fallback).
- **AGENTS.md**: **Refactor analysis intake (2026-03-23)** — complementary deep pass: large tests (**`handlers_test`**, **`diagnostic_test`**, **`websocket/server_test`**, **`results/handler_test`**, **`registry/handler_test`**), **`internal/metrics`**, **`pkg/diagnostic`**, **`cmd/client`** **`config`/`api`**, **`router`/`handlers`**, web **HTTP speed** further dedupe, **`cmd/check`**; **Live Queue** **`20260323-refactor-01`**..**`10`** with checks.
- **AGENTS.md**: **Refactor analysis intake (2026-03-22)** — deep pass: test-suite **LOC** hotspots (**`router_test`**, **`store_test`**, **`manager_test`**, **`e2e_test`**), **`cmd/client`** engine/formatter, **`internal/registry`**, optional **`speedtest.go`**, **web** **`ui`/`openbyte`**, **MCP/loadtest**; **Live Queue** **`20260322-refactor-01`**..**`09`** with checks.
- **AGENTS.md**: **Refactor analysis intake (2026-03-20)** + backlog **`20260320-refactor-14`**..**`16`** (**`cmd/client`**, **`web`**, **`internal/stream` `Server`**) **Done**.
- **AGENTS.md**: Architecture § Performance **advanced telemetry** policy (**`20260320-perf-03`**): server/internal-first, minimal default Web UI, explicit opt-in for user-visible detail; implementation remains deferred.
- **Benchmarks**: **`internal/api`** (**`respondJSON`**, **`validateMetricsPayload`**, **`normalizeHost`**) and **`internal/jsonbody`** (**`DecodeSingleObject`**); **`Makefile`** **`perf-bench`** runs them with existing unit benches; **AGENTS.md** documents **`benchstat`** comparison (manual).
- **Playwright**: **`playwright.config.js`** sets **`workers`** to **`2`** when **`GITHUB_ACTIONS`** is set (GitHub-hosted runners); optional **`PLAYWRIGHT_WORKERS`** override.
- **AGENTS.md**: documented **CI** vs **nightly** race-detector matrix (**`-short`**/**`-p 1`** on **`main`** vs full **`go test -race ./...`** nightly); comments in **`ci.yml`** / **`nightly.yml`**.
- **CI**: **`govulncheck`** in **`checks`** (`go run golang.org/x/vuln/cmd/govulncheck@latest ./...`); **Redocly** pinned as **`@redocly/cli@2.18.1`** with **`bun run lint:openapi`** (replaces cold **`npx @redocly/cli`** per run); **`Makefile`** **`lint-openapi`** for local parity.

### Fixed

- **Web / server**: **`GET /api/v1/servers`** **`api_endpoint`** no longer uses **`0.0.0.0`** (default **`BIND_ADDRESS`**) for browser URLs — uses the request **`Host`** when the bind address is unspecified so the UI stays same-origin (**`localhost`** vs **`127.0.0.1`** preserved). **`internal/api/router_static.go`** allowlists **`speedtest-http-{download,shared,upload}.js`** (split modules were **404**); regression test **`TestRouterStaticServesSpeedtestHTTPModules`**. **`README.md`** Configuration notes: **`Host`** / **`PUBLIC_HOST`** for local vs **`127.0.0.1`**-only bind.
- **SonarQube** (targets **2026-03-20** OPEN list): **`[[`** conditionals in **`scripts/deploy/*.sh`**; **`TestDecodeSingleObject*`** names in **`internal/jsonbody/decode_test.go`**; **`go:S1192`** via shared format / path constants in tests; **`playwright.config.js`** **`resolvePlaywrightWorkers()`** (no nested ternary); **`execContexter`** in **`internal/results/store_migrate.go`**; **`test/e2e/ui/basic.spec.js`** fetch mock: **`init?.signal`** / **`signal?.`** for **`javascript:S6582`**; success toast **`<output>`** in **`web/index.html`** + assertion update. **AGENTS.md** Sonar snapshot updated.
- **CI**: `build-push` / `deploy` no longer skipped on docs-only `main` pushes (removed `paths-filter` `docker` gate from image job; `changes` still used for PR Playwright gating).
- **Release**: `deploy` no longer gated on `image_pushed` job output (boolean→string mismatch with `'true'` could skip SSH deploy after a successful `release` job); use `needs.release.result == 'success'` like CI `deploy`.

### Changed

- **cmd/client**: replaced monolithic **`run.go`** with **`run_stream.go`**, **`run_http.go`**, **`run_progress.go`**, **`run_results.go`**; split **`http_engine.go`** into **`http_engine_download.go`**, **`http_engine_upload.go`**, **`http_engine_misc.go`** (ping + stream helpers) + slim core **`http_engine.go`**.
- **pkg/client**: replaced **`client_http.go`** with **`client_{check,speedtest,diagnose,health,latency,download,upload}.go`** + slim **`client.go`** (same exported API).
- **test/unit/api**: split **`speedtest_test.go`** into **`speedtest_helpers_test.go`**, **`speedtest_download_test.go`**, **`speedtest_upload_test.go`**, **`speedtest_ping_test.go`** (same **`package api_test`**; no assertion changes).
- **internal/stream**: split TCP workload path out of **`server.go`** into **`server_tcp.go`** (**`server.go`** keeps **`NewServer`**, **`Close`**, buffer pool, **`isTimeoutError`** for UDP + TCP); **`server_udp.go`** unchanged.
- **Web**: split **`download.js`** into **`download-platform.js`**, **`download-github.js`**, and slim **`download.js`**; split **`network.js`** into **`network-helpers.js`**, **`network-health.js`**, and slim **`network.js`** (**`getHealthURL`** still exported from **`network.js`**); **`internal/api/router_static.go`** allowlists the new **`*.js`** assets.
- **cmd/client**: split former **`cli.go`** into **`cli_flags.go`**, **`cli_usage.go`**, **`cli_validate.go`**, **`cli_servers.go`** (behavior-preserving; same **`package client`**).
- **Nightly** (`nightly.yml`): **`make perf-bench`** runs on each schedule unless repo variable **`PERF_BENCH`** is **`false`** (replaces **`PERF_SMOKE == 'true'`** gate); **`make perf-leakcheck`** still **`vars.LEAK_PROFILE_SMOKE == 'true'`**.
- **CI** (`ci.yml`): **`cancel-in-progress`** only when **`github.event_name == 'pull_request'`** — **`main`** / tags / **`workflow_dispatch`** no longer cancel an in-flight run (avoids aborting **`deploy`**); next run **queues** on the same ref. **`AGENTS.md`** documents tradeoff (possible **`main`** backlog).
- **cmd/server**: split monolithic `main.go` into `flags.go` and `runtime.go` (behavior-preserving; easier navigation).
- **internal/config**: split `env.go` into `env_helpers.go`, `env_core.go`, and `env_extended.go` (same `LoadFromEnv` behavior).
- **CI / release**: `deploy` jobs use shared **`scripts/deploy/`** bash (`validate_env`, `sync_compose`, `deploy_remote`) instead of duplicated inline shell.
- **internal/results**: split SQLite store into `store_migrate.go`, `store_id.go`, `store_crud.go`, `store_cleanup.go` + slim `store.go` (no API change).
- **Web**: split `speedtest-http.js` into `speedtest-http-shared.js`, `speedtest-http-download.js`, `speedtest-http-upload.js` (barrel keeps same exports).
- **internal/api**: split `router_middleware.go` into `router_middleware_{ratelimit,cors,logging,security}.go` (same middleware chain).
- **internal/stream**: split `manager.go` into `manager_streams.go`, `manager_cleanup.go`, `manager_broadcast.go` (same `Manager` API).

## [0.8.0] - 2026-03-19

### Security

- Go **1.26.1** toolchain baseline; Docker builder image **golang:1.26.1-alpine**.
- Transitive **github.com/buger/jsonparser** updated to **v1.1.2** (addresses Dependabot advisory).

### Changed

- **internal/jsonbody**: shared single-object JSON request decoding for API and results handlers.
- **internal/websocket**: split large `server.go` into focused files (origin/CORS, broadcast, limits, ping, lifecycle, message types).
- **internal/api**: split `speedtest` and `handlers` across `speedtest_*.go`, `handlers_meta.go`, `handlers_stream.go`.
- **pkg/client**: HTTP/latency/download/upload helpers moved to **client_http.go** (no exported API renames).
- **CI**: race tests also run on **workflow_dispatch** for `main`; **AGENTS.md** documents recovery when a push-triggered run is stuck.

### Dependencies

- **golang.org/x/term** v0.41.0, **modernc.org/sqlite** v1.47.0, **github.com/mark3labs/mcp-go** v0.45.0 (and transitive updates).
- Routine **GitHub Actions** version bumps via Dependabot.

[Unreleased]: https://github.com/SaveEnergy/openbyte/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/SaveEnergy/openbyte/compare/v0.7.0...v0.8.0
