# Changelog

Notable user-facing changes are documented here. Internal refactor diaries belong
in Git history and pull requests, not release notes.

## [Unreleased]

### Added

- **Privacy controls**: a localized `/privacy` technical summary
  documents request IPs, sharing, retention, logs, recipients, and device
  storage. `PRIVACY_URL` can redirect to the operator-specific GDPR notice.
  Recent-result persistence is now an explicit, default-off browser choice
  rather than an automatic write.
- **Operator legal notice**: `IMPRESSUM_URL` redirects to the operator's
  separate legal notice and reveals its footer link.

- **Environment-driven visual branding**: operators can provide accessible
  dark/light primary and secondary colors plus a bounded local header logo;
  both the speed-test and shared-result pages apply the branding before paint.
- **English and German UI**: the speed test and shared-result pages now detect
  the browser language, provide a persistent reload-based selector that exposes
  the resolved system language, localize accessible status text, and format
  results and history for the chosen locale. Language preferences stay on the
  device instead of being propagated through navigation and share links.
- **Test legibility**: the testing screen now shows a determinate progress ring
  (ramp windows + measure countdown reported by the worker), a phase stepper
  (Ping → Download → Upload) that keeps completed phase results visible as
  chips, and a live throughput sparkline under the speed number.
- **Result context**: a loaded-latency advisory, a colored bufferbloat grade
  badge, and a "What do these numbers mean?" disclosure explaining the
  secondary metrics without assigning a subjective connection label.
- **Recent tests**: users can explicitly enable storage of the last runs in
  `localStorage`; disabling it deletes the history (`web/history.js`).
- **Preferences menu**: one header disclosure exposes language, explicit
  system/light/dark choices, and result storage on every page.

### Changed

- **Header polish**: removed the underline accent from the wordmark, moved
  device preferences behind one compact settings trigger, and the optional
  brand logo is requested only on branded deployments (no more
  `/branding/logo` 404 in the console). Unresolvable `v4.`/`v6.` probe hosts
  are remembered only in page memory, never device storage.
- **Official container contract**: the published image and bundled Compose now
  use plain HTTP on internal port `8080` and persist only at `/app/data`, while
  runtime defaults come from the binary. Custom data-path, direct-TLS, HTTP/2,
  and pprof deployments now require an explicit container override.
- **Server configuration**: deployment settings are now environment-only; the
  pre-release command-line configuration flags were removed.
- **Compose setup**: local source builds now layer a minimal override on the
  published-image base; the unused Traefik dashboard and upload-specific routers
  were removed.
- **Deployment**: releases now deploy through one verified SSH connection while
  retaining checksum, image, health, and rollback checks.
- **Transfer concurrency**: replaced the inferred capacity heuristic with the
  explicit `MAX_CONCURRENT_TRANSFERS` limit, defaulting to the same 200
  download streams and 200 upload streams. Migrate a previous capacity value
  with `max(old*8, 50)`.
- **Ping response**: `/api/v1/ping` now returns only `client_ip` by default;
  `?meta=1` also returns `server_name`. Removed `pong`, `timestamp`, and the
  redundant `ipv6` flag; the UI derives the address family locally while keeping
  eager same-origin, IPv4, and IPv6 discovery.
- **Throughput reporting**: download and upload now report measured payload
  goodput without adding an estimated HTTP/1 protocol-overhead multiplier.
- **One-tap share**: tapping Share now saves the result and copies the link in
  a single gesture (previously it required two taps).
- **Cancel behavior**: cancelling aborts the active transfer, discards the
  incomplete run, and returns to the ready screen without saving history or a
  shareable result.
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
- **Shared result page**: aligned with the live results view — loaded-latency
  advisory, colored bufferbloat badge, metric explanations, and the theme
  toggle now appear on `/results/{id}` too. The explanations live in a shared
  `stats-help.js` module used by both pages.
- **Release archives**: Linux and macOS builds are published for amd64 and arm64
  as `.tar.gz` files with one checksum manifest.

### Security

- Raised the Go baseline to 1.26.5 for the `crypto/tls` fix tracked as
  GO-2026-5856; CI follows the latest 1.26.x patch and Dependabot watches Docker
  base images.

### Fixed

- Prevented browsers from synthesizing unavailable display-font weights and
  made native form controls inherit the bundled DM Sans font.
- Required `MAX_TEST_DURATION` to be whole seconds of at least `1s`, and bounded
  an omitted download duration by the configured maximum.

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
- Removed the obsolete multi-server Compose example; use independent deployments
  with explicit URLs when comparing regions.
- Removed the unadvertised curl installer; release archives and their checksum
  manifest remain available for manual installation.

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
