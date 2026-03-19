# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **AGENTS.md**: Architecture § Performance **advanced telemetry** policy (**`20260320-perf-03`**): server/internal-first, minimal default Web UI, explicit opt-in for user-visible detail; implementation remains deferred.
- **Benchmarks**: **`internal/api`** (**`respondJSON`**, **`validateMetricsPayload`**, **`normalizeHost`**) and **`internal/jsonbody`** (**`DecodeSingleObject`**); **`Makefile`** **`perf-bench`** runs them with existing unit benches; **AGENTS.md** documents **`benchstat`** comparison (manual).
- **Playwright**: **`playwright.config.js`** sets **`workers`** to **`2`** when **`GITHUB_ACTIONS`** is set (GitHub-hosted runners); optional **`PLAYWRIGHT_WORKERS`** override.
- **AGENTS.md**: documented **CI** vs **nightly** race-detector matrix (**`-short`**/**`-p 1`** on **`main`** vs full **`go test -race ./...`** nightly); comments in **`ci.yml`** / **`nightly.yml`**.
- **CI**: **`govulncheck`** in **`checks`** (`go run golang.org/x/vuln/cmd/govulncheck@latest ./...`); **Redocly** pinned as **`@redocly/cli@2.18.1`** with **`bun run lint:openapi`** (replaces cold **`npx @redocly/cli`** per run); **`Makefile`** **`lint-openapi`** for local parity.

### Fixed

- **SonarQube** (targets **2026-03-20** OPEN list): **`[[`** conditionals in **`scripts/deploy/*.sh`**; **`TestDecodeSingleObject*`** names in **`internal/jsonbody/decode_test.go`**; **`go:S1192`** via shared format / path constants in tests; **`playwright.config.js`** **`resolvePlaywrightWorkers()`** (no nested ternary); **`execContexter`** in **`internal/results/store_migrate.go`**; **`test/e2e/ui/basic.spec.js`** fetch mock: **`init?.signal`** / **`signal?.`** for **`javascript:S6582`**; success toast **`<output>`** in **`web/index.html`** + assertion update. **AGENTS.md** Sonar snapshot updated.
- **CI**: `build-push` / `deploy` no longer skipped on docs-only `main` pushes (removed `paths-filter` `docker` gate from image job; `changes` still used for PR Playwright gating).
- **Release**: `deploy` no longer gated on `image_pushed` job output (boolean→string mismatch with `'true'` could skip SSH deploy after a successful `release` job); use `needs.release.result == 'success'` like CI `deploy`.

### Changed

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
