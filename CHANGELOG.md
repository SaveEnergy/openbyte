# Changelog

Notable user-facing changes are documented here. Internal refactor diaries belong
in Git history and pull requests, not release notes.

## [Unreleased]

### Added

- **English and German UI**: the speed test and shared-result pages now detect
  the browser language, provide a persistent reload-based selector that exposes
  the resolved system language, localize accessible status text, and format
  results and history for the chosen locale. Language preferences stay on the
  device instead of being propagated through navigation and share links.
- **Test legibility**: the testing screen now shows a determinate progress ring
  (ramp windows + measure countdown reported by the worker), a phase stepper
  (Ping → Download → Upload) that keeps completed phase results visible as
  chips, and a live throughput sparkline under the speed number.
- **Result interpretation**: a concise verdict after the primary speed cards,
  a separate loaded-latency advisory, a colored bufferbloat grade badge, and a
  "What do these numbers mean?" disclosure explaining the secondary metrics.
- **Recent tests**: the last runs are stored in `localStorage` and listed on
  the results screen (`web/history.js`).
- **Manual theme toggle**: header button cycles system → light → dark and
  persists per device (`web/theme.js`); applies on the main, shared-result,
  and shared-result pages.

### Changed

- **Throughput reporting**: download and upload now report measured payload
  goodput without adding an estimated HTTP/1 protocol-overhead multiplier.
- **One-tap share**: tapping Share now saves the result and copies the link in
  a single gesture (previously it required two taps).
- **Cancel keeps partial results**: cancelling after the download phase shows
  latency + download with a "cancelled early" notice instead of discarding
  everything; sharing is disabled for partial runs.
- **Offline handling**: the server health check re-runs every 30 s while idle,
  and an offline server disables the GO button instead of failing on click.
- **Light theme & toast polish**: light mode gains card borders/shadows, and
  toasts use fixed high-contrast colors in both themes.
- **Mobile results layout**: the four secondary stats render as a 2×2 grid on
  small screens, while download and upload remain side by side.
- **Brand and localization polish**: restored mint as the stable openByte brand
  color without reusing it for low-contrast light-theme text, grouped language
  and theme preferences into one compact control, shortened German copy, and
  stopped exposing adaptive-stream jargon in the test UI.
- **Shared result page**: aligned with the live results view — verdict line,
  colored bufferbloat badge, metric explanations, and the theme toggle now
  appear on `/results/{id}` too. The explanations live in a shared
  `stats-help.js` module used by both pages.

### Security

- Raised the Go baseline to 1.26.5 for the `crypto/tls` fix tracked as
  GO-2026-5856; CI follows the latest 1.26.x patch and Dependabot watches Docker
  base images.

### Fixed

- Prevented browsers from synthesizing unavailable display-font weights and
  made native form controls inherit the bundled DM Sans font.
- Partial results now label download latency explicitly and omit the generic
  bufferbloat grade and warning because upload was not measured.
- Required `MAX_TEST_DURATION` to be whole seconds of at least `1s`, and bounded
  an omitted download duration by the configured maximum.

### Changed

- Simplified the browser transfer state machines, one-shot worker protocol, and
  instrument animation while retaining adaptive streams, result sharing, and
  eager same-origin/IPv4/IPv6 address discovery.
- Made discovered public IPv4/IPv6 addresses visible before starting a test;
  the same-origin bootstrap ping now supplies server-name metadata while
  measurement and address-discovery pings retain their lean response.
- Made server configuration environment-only, replaced the custom logger with
  `log/slog`, and consolidated router/result HTTP setup without changing the
  persisted result schema or share URLs.
- Consolidated duplicated source/GHCR Compose definitions into a published-image
  base plus a minimal local-build override; removed the unused Traefik dashboard
  and redundant upload-specific routers.
- Deployment now verifies, syncs, and starts the remote release through one SSH
  connection while retaining fingerprint, checksum, image, health, and rollback
  checks.
- Main workflow dispatches now follow the same build/deploy path as main pushes;
  branch dispatches run checks only. Nightly is one full race gate, while perf
  and leak profiling remain explicit local tools.
- Release archives now match the installer-supported platforms: Linux and macOS
  on amd64/arm64, all as `.tar.gz` with one checksum manifest.
- Curated performance benchmarks around transfer, gzip, JSON, ping, and SQLite
  behavior; removed unused baseline-comparison plumbing.
- Reduced unit-test wall-clock waits without reducing behavioral coverage.

### Removed

- Removed the browser API quick reference, configurable generic CORS, and
  `/api/v1/version`. `api/openapi.yaml` is the canonical contract, and only the
  ping route permits cross-origin discovery requests.
- Removed the CLI speed-test clients, Go SDK, diagnostics package, and shared
  client transfer helpers. The `openbyte` binary now runs only the server; use
  the browser or HTTP API for tests and automation.
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
