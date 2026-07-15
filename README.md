<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/openbyte-wordmark-dark.svg">
    <img src="docs/assets/openbyte-wordmark-light.svg" alt="openByte" width="460">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/saveenergy/openbyte/actions/workflows/ci.yml">
    <img src="https://github.com/saveenergy/openbyte/actions/workflows/ci.yml/badge.svg" alt="CI">
  </a>
  <a href="https://github.com/saveenergy/openbyte/releases">
    <img src="https://img.shields.io/github/v/release/saveenergy/openbyte?sort=semver" alt="Latest Release">
  </a>
  <a href="https://github.com/saveenergy/openbyte/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/saveenergy/openbyte" alt="License">
  </a>
  <a href="https://go.dev/">
    <img src="https://img.shields.io/github/go-mod/go-version/saveenergy/openbyte" alt="Go Version">
  </a>
</p>

High-performance browser-first network speed test server for multi-gigabit links. HTTP streaming, adaptive multi-stream testing, latency, jitter, and bufferbloat measurement.

## Quick Start

### Build & Run

```bash
make build
./bin/openbyte

# Server configuration is environment-only
SERVER_NAME="Frankfurt 25G" ./bin/openbyte
```

### Docker

```bash
# Published edge image
cd docker && docker compose up -d

# Build the current checkout locally
cd docker && docker compose -f docker-compose.yaml -f docker-compose.local.yaml up -d --build

# Published image with Traefik reverse proxy
cd docker && docker compose -f docker-compose.yaml -f docker-compose.traefik.yaml up -d
```

### Web Interface

Open `http://localhost:8080` — minimal fast.com-inspired UI with adaptive stream ramping, Web Worker transfer loops, and real-time speed visualization.

## Features

- **Protocols**: HTTP streaming for the Web UI and API
- **Test Types**: Download, Upload
- **Metrics**: Throughput, idle latency, jitter, loaded latency, bufferbloat
- **Public IP**: IPv4 and IPv6 discovery shown immediately, before a speed test
- **Adaptive web test**: Browser UI ramps parallel HTTP streams automatically, then measures with the stream count that saturated the path; transfer loops run in a Web Worker to keep the UI responsive
- **Automation**: OpenAPI-documented HTTP API

## Measurement Methodology

The browser client implements:

- Adaptive Web Worker stream ramping plus dynamic warm-up with throughput stabilization detection
- Baseline latency measurement before each test
- Metrics reset after warm-up for accurate results
- Median latency reporting with IQR outlier filtering

## Configuration

### Server Environment Variables

| Variable              | Default           | Description                                                        |
| --------------------- | ----------------- | ------------------------------------------------------------------ |
| `PORT`                | 8080              | HTTP API port                                                      |
| `SERVER_NAME`         | `openByte Server` | Display name in bootstrap ping metadata, the Web UI, and saved results |
| `BRAND_PRIMARY_COLOR_DARK` / `BRAND_PRIMARY_COLOR_LIGHT` | — | Primary action/download color pair, in exact `#RRGGBB` form |
| `BRAND_SECONDARY_COLOR_DARK` / `BRAND_SECONDARY_COLOR_LIGHT` | — | Secondary/upload color pair, in exact `#RRGGBB` form |
| `BRAND_LOGO_PATH`     | —                 | PNG or JPEG logo path readable by the server (maximum 1 MiB)       |
| `CAPACITY_GBPS`       | 25                | Server link capacity; HTTP concurrency limits auto-scale from this |
| `MAX_CONCURRENT_PER_IP` | 64              | Concurrent speed-test streams allowed per client IP and direction  |
| `RATE_LIMIT_PER_IP`   | 100               | Per-IP requests/minute for shared-result routes                     |
| `GLOBAL_RATE_LIMIT`   | 1000              | Global requests/minute for shared-result routes                     |
| `TRUST_PROXY_HEADERS` | false             | Trust proxy headers for client IP                                  |
| `TRUSTED_PROXY_CIDRS` | —                 | Comma-separated trusted proxy CIDRs                                |
| `WEB_ROOT`            | _(embedded)_      | Override path to static web assets (for development)               |
| `MAX_TEST_DURATION`   | `300s`            | Maximum test duration (whole seconds in Go duration format, at least `1s`) |
| `DATA_DIR`            | `./data`          | Path to SQLite database directory                                  |
| `MAX_STORED_RESULTS`  | 10000             | Maximum stored results; results older than 90 days are also purged  |
| `BIND_ADDRESS`        | `0.0.0.0`         | Address to bind listeners                                          |
| `PPROF_ENABLED`       | false             | Enable pprof profiling server                                      |
| `PPROF_ADDR`          | `127.0.0.1:6060`  | pprof server listen address                                        |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` | —      | Serve TLS with this PEM pair; both values are required              |
| `TLS_AUTO_GEN`        | false             | Generate an ephemeral self-signed localhost certificate for development |
| `HTTP2_ENABLED`       | true              | Enable HTTP/2 when the server is serving TLS                        |

Notes:

- If you bind `127.0.0.1` only, open the UI at `http://127.0.0.1:PORT`.
- Configure public DNS and reverse-proxy routing outside openByte; saved-result URLs are relative.
- For reverse proxy deployments, set `TRUST_PROXY_HEADERS=true` and `TRUSTED_PROXY_CIDRS` to the proxy IP ranges.
- `/api/v1/ping` is the only cross-origin API: it allows any origin so the UI can probe dedicated IPv4/IPv6 hostnames. Other API routes are same-origin.
- There is no `/api/v1/version` route. The UI requests `/api/v1/ping?meta=1` during bootstrap to load `SERVER_NAME`; measurement and address-discovery pings keep the smaller default response.
- If running behind a reverse proxy, allow more than the browser's adaptive 64 MiB maximum request payload and disable request buffering for `/api/v1/upload` to avoid upload failures or inflated results.
- Server configuration uses environment variables only; `openbyte --help` lists command-only options.

Brand colors are optional, but each dark/light pair must be set together.
Primary colors must meet a 4.5:1 contrast ratio and secondary colors a 3:1
ratio against the corresponding built-in surfaces; invalid combinations fail
startup instead of silently producing an unreadable UI. A custom logo replaces
the header wordmark on both the speed-test and shared-result pages. It is read
once at startup, must be a bounded PNG or JPEG, and does not replace the
favicon, page metadata, or upstream attribution. Use logo artwork that remains
legible in both light and dark themes.

### Deployment With Environment Variables

```bash
# docker run
docker run --rm -p 8080:8080 \
  -e SERVER_NAME="Frankfurt 25G" \
  ghcr.io/saveenergy/openbyte:latest

# Branded deployment (the light values are deliberately darker for contrast)
mkdir -p branding
cp /path/to/company-logo.png branding/logo.png
docker run --rm -p 8080:8080 \
  -v "$PWD/branding:/app/branding:ro" \
  -e BRAND_LOGO_PATH=/app/branding/logo.png \
  -e BRAND_PRIMARY_COLOR_DARK="#66E3FF" \
  -e BRAND_PRIMARY_COLOR_LIGHT="#00677A" \
  -e BRAND_SECONDARY_COLOR_DARK="#FFB45C" \
  -e BRAND_SECONDARY_COLOR_LIGHT="#9A4D00" \
  ghcr.io/saveenergy/openbyte:latest

# Docker Compose reads the same values from docker/.env and mounts
# BRAND_ASSETS_DIR read-only at /app/branding.
```

### IPv4/IPv6 Detection

The web UI displays both client IPv4 and IPv6 addresses using dedicated single-stack subdomains. To enable:

1. Add a DNS **A-only** record for `v4.<your-domain>` → server IPv4 (no AAAA).
2. Add a DNS **AAAA-only** record for `v6.<your-domain>` → server IPv6 (no A).
3. Include both in your Traefik host rule or reverse proxy config.

See [Deployment Guide](DEPLOYMENT.md#ipv4-and-ipv6-discovery) for details.

## Testing

### UI E2E (Playwright)

```bash
bunx playwright test
# or
make test-ui
```

Playwright starts and owns a local server on `127.0.0.1:8080`; the port must be free.

## Documentation

- [Architecture](ARCHITECTURE.md) — System design and components
- [`api/openapi.yaml`](api/openapi.yaml) — canonical API contract
- [Deployment Guide](DEPLOYMENT.md) — Production deployment
- [Performance Guide](PERFORMANCE.md) — Profiling, load testing, perf checks

The server does not duplicate the OpenAPI contract as a browser page.

## Project Structure

```
cmd/
  openbyte/   # Server binary entry point
  server/     # Server implementation
internal/
  api/        # REST API + HTTP speed test handlers
  config/     # Configuration
  results/    # SQLite results store
docker/       # Docker + Compose configurations
web/          # Web UI (embedded in binary)
  fonts/      # Self-hosted font files
test/         # Go unit/API and Playwright tests
```

## License

MIT License — see [LICENSE](LICENSE)

## Acknowledgments

Inspired by [Breitbandmessung](https://github.com/breitbandmessung) from the German Federal Network Agency (BEREC Net Neutrality Reference Measurement System).
