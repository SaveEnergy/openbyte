# Changelog

Notable user-facing changes are documented here. Internal refactor diaries belong
in Git history and pull requests, not release notes.

## [Unreleased]

### Security

- Raised the Go baseline to 1.26.5 for the `crypto/tls` fix tracked as
  GO-2026-5856; CI follows the latest 1.26.x patch and Dependabot watches Docker
  base images.

### Fixed

- Removed the Go SDK's 60-second global HTTP timeout so long speed-test streams
  use caller and operation contexts; health and the full latency phase are each
  bounded to 10 seconds.
- Required `MAX_TEST_DURATION` to be whole seconds of at least `1s`, and bounded
  an omitted download duration by the configured maximum.

### Changed

- Simplified the browser transfer state machines, one-shot worker protocol, and
  instrument animation while retaining adaptive streams, result sharing, and
  eager same-origin/IPv4/IPv6 address discovery.
- Made discovered public IPv4/IPv6 addresses visible before starting a test;
  ping readiness remains authoritative if version metadata is rate-limited.
- Made server configuration environment-only, replaced the custom logger with
  `log/slog`, and consolidated router/result HTTP setup without changing the
  persisted result schema or share URLs.
- Consolidated the source and GHCR Traefik configurations into one overlay.
- Deployment now verifies, syncs, and starts the remote release through one SSH
  connection while retaining fingerprint, checksum, image, health, and rollback
  checks.
- Removed duplicate tag CI and nightly E2E executions. Release and nightly retain
  their full validation gates.
- Curated performance benchmarks around transfer, gzip, JSON, ping, and SQLite
  behavior; removed unused baseline-comparison plumbing.
- Reduced unit-test wall-clock waits without reducing behavioral coverage.

### Removed

- Removed the full `openbyte client`, its YAML configuration, terminal output
  formats, Docker target, and CLI throughput harness. Use the browser, HTTP API,
  Go SDK, or `openbyte check --json`.
- Removed the compatibility-only `diagnostics` field from result creation;
  result sharing and persisted result fields are unchanged.
- Removed unused `PUBLIC_HOST`, nonfunctional `LOG_LEVEL`, periodic runtime
  statistics, and `/debug/runtime-metrics`; loopback pprof remains available.
- Removed the unwired metrics collectors, always-zero legacy CLI JSON fields,
  obsolete multi-server Compose example, and abandoned autoresearch harness.

## [0.10.2] - 2026-05-04

### Fixed

- Corrected browser upload accounting so only bytes overlapping the measured
  window contribute to upload throughput.

## [0.10.1] - 2026-05-04

### Fixed

- Aligned release race coverage with CI while retaining the full race suite in
  nightly runs.

## [0.10.0] - 2026-05-04

### Added

- Added adaptive browser stream ramping and Web Worker transfer loops.
- Added the in-app `/api.html` quick reference.

### Removed

- Removed manual browser stream/duration settings, TCP/UDP testing, websocket
  stream APIs, direct data ports, the downloads page, and the load-test command.
  The runtime is HTTP-only and exposes port 8080.

## [0.9.1] - 2026-05-03

### Added

- Added `SERVER_NAME` for the UI, version response, and shared results.

## [0.9.0] - 2026-05-03

### Removed

- Removed the MCP server, registry, server selector, legacy client aliases, and
  standalone integration guide. Integrations use the HTTP/OpenAPI contract.

## [0.8.0] - 2026-03-19

### Changed

- Moved shared JSON decoding and HTTP client helpers into focused packages and
  upgraded the Go and dependency baselines.

[Unreleased]: https://github.com/SaveEnergy/openbyte/compare/v0.10.2...HEAD
[0.10.2]: https://github.com/SaveEnergy/openbyte/compare/v0.10.1...v0.10.2
[0.10.1]: https://github.com/SaveEnergy/openbyte/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/SaveEnergy/openbyte/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/SaveEnergy/openbyte/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/SaveEnergy/openbyte/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/SaveEnergy/openbyte/compare/v0.7.0...v0.8.0
