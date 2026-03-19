# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **CI**: `build-push` / `deploy` no longer skipped on docs-only `main` pushes (removed `paths-filter` `docker` gate from image job; `changes` still used for PR Playwright gating).
- **Release**: `deploy` no longer gated on `image_pushed` job output (boolean→string mismatch with `'true'` could skip SSH deploy after a successful `release` job); use `needs.release.result == 'success'` like CI `deploy`.

### Changed

- **cmd/server**: split monolithic `main.go` into `flags.go` and `runtime.go` (behavior-preserving; easier navigation).
- **internal/config**: split `env.go` into `env_helpers.go`, `env_core.go`, and `env_extended.go` (same `LoadFromEnv` behavior).
- **CI / release**: `deploy` jobs use shared **`scripts/deploy/`** bash (`validate_env`, `sync_compose`, `deploy_remote`) instead of duplicated inline shell.
- **internal/results**: split SQLite store into `store_migrate.go`, `store_id.go`, `store_crud.go`, `store_cleanup.go` + slim `store.go` (no API change).
- **Web**: split `speedtest-http.js` into `speedtest-http-shared.js`, `speedtest-http-download.js`, `speedtest-http-upload.js` (barrel keeps same exports).
- **internal/api**: split `router_middleware.go` into `router_middleware_{ratelimit,cors,logging,security}.go` (same middleware chain).

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
