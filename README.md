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

High-performance browser-first network speed test server capable of 25 Gbit/s sustained throughput. HTTP streaming, adaptive multi-stream testing, latency, jitter, and bufferbloat measurement.

## Quick Start

### Build & Run

```bash
make build
./bin/openbyte server

# With server flags (flags override env values when set)
./bin/openbyte server --public-host speedtest.example.com
```

### CLI Client

```bash
./bin/openbyte client -d download -t 30        # 30-second download test
./bin/openbyte client -S https://speed.example.com
./bin/openbyte client https://speed.example.com
./bin/openbyte client -d upload                # HTTP streaming upload
```

### Docker

```bash
# Single server
cd docker && docker compose up -d

# With Traefik reverse proxy
cd docker && docker compose -f docker-compose.yaml -f docker-compose.traefik.yaml up -d

# Multi-server deployment
cd docker && docker compose -f docker-compose.yaml -f docker-compose.multi.yaml --profile multi up -d
```

### Web Interface

Open `http://localhost:8080` — minimal fast.com-inspired UI with adaptive stream ramping, Web Worker transfer loops, and real-time speed visualization.

## Features

- **Protocols**: HTTP streaming for the Web UI, API, SDK, and CLI
- **Test Types**: Download, Upload
- **Metrics**: Throughput, idle latency, jitter, loaded latency, bufferbloat
- **RTT**: Baseline and during-test round-trip time measurement
- **Network Info**: Client IP and IPv6 detection
- **Adaptive web test**: Browser UI ramps parallel HTTP streams automatically, then measures with the stream count that saturated the path; transfer loops run in a Web Worker to keep the UI responsive
- **Multi-server**: Deploy globally, test against nearest server
- **Output**: JSON, plain text, interactive CLI

## Measurement Methodology

Uses BEREC-compliant measurement practices:

- Adaptive Web Worker stream ramping plus dynamic warm-up with throughput stabilization detection (web UI); fixed warm-up via `--warmup` (CLI)
- Baseline RTT measurement (10 samples before test)
- Metrics reset after warm-up for accurate results
- Statistical reporting with P50, P95, P99 percentiles

## Configuration

### Server Environment Variables

| Variable              | Default           | Description                                                        |
| --------------------- | ----------------- | ------------------------------------------------------------------ |
| `PORT`                | 8080              | HTTP API port                                                      |
| `SERVER_NAME`         | `openByte Server` | Display name shown in the Web UI and saved results                 |
| `PUBLIC_HOST`         | —                 | Public hostname/IP                                                 |
| `CAPACITY_GBPS`       | 25                | Server link capacity; HTTP concurrency limits auto-scale from this |
| `RATE_LIMIT_PER_IP`   | 100               | Rate limit per IP per minute                                       |
| `GLOBAL_RATE_LIMIT`   | 1000              | Global rate limit per minute                                       |
| `TRUST_PROXY_HEADERS` | false             | Trust proxy headers for client IP                                  |
| `TRUSTED_PROXY_CIDRS` | —                 | Comma-separated trusted proxy CIDRs                                |
| `ALLOWED_ORIGINS`     | `*`               | Comma-separated CORS allowed origins                               |
| `WEB_ROOT`            | _(embedded)_      | Override path to static web assets (for development)               |
| `MAX_TEST_DURATION`   | `300s`            | Maximum test duration (Go duration format)                         |
| `DATA_DIR`            | `./data`          | Path to SQLite database directory                                  |
| `MAX_STORED_RESULTS`  | 10000             | Maximum stored test results (older results auto-purged)            |
| `BIND_ADDRESS`        | `0.0.0.0`         | Address to bind listeners                                          |
| `PPROF_ENABLED`       | false             | Enable pprof profiling server                                      |
| `PPROF_ADDR`          | `127.0.0.1:6060`  | pprof server listen address                                        |
| `PERF_STATS_INTERVAL` | —                 | Log runtime stats at this interval (e.g. `10s`)                    |

Notes:

- If you bind `127.0.0.1` only, open the UI at `http://127.0.0.1:PORT`, or set `PUBLIC_HOST` for stable generated URLs.
- For reverse proxy deployments, set `TRUST_PROXY_HEADERS=true` and `TRUSTED_PROXY_CIDRS` to the proxy IP ranges.
- Default CORS allows all origins; set `ALLOWED_ORIGINS` to restrict (supports `*` and `*.example.com`).
- If running behind a reverse proxy, increase max request body size (e.g. 35MB) and disable request buffering for `/api/v1/upload` to avoid upload failures or inflated results.
- Server command supports flags for deployment (`openbyte server --help`). If both env var and flag are set, the flag wins.

### Deployment With Server Flags

```bash
# docker run
docker run --rm -p 8080:8080 \
  ghcr.io/saveenergy/openbyte:latest \
  server --public-host speed.example.com

# docker compose service command override
# command: ["server", "--public-host=speed.example.com"]
```

### IPv4/IPv6 Detection

The web UI displays both client IPv4 and IPv6 addresses using dedicated single-stack subdomains. To enable:

1. Add a DNS **A-only** record for `v4.<your-domain>` → server IPv4 (no AAAA).
2. Add a DNS **AAAA-only** record for `v6.<your-domain>` → server IPv6 (no A).
3. Include both in your Traefik host rule or reverse proxy config.

See [Deployment Guide](DEPLOYMENT.md#ipv4ipv6-detection) for details.

### Client Configuration

`~/.config/openbyte/config.yaml`:

```yaml
server_url: https://speedtest.example.com
chunk_size: 1048576
```

## Testing

### UI E2E (Playwright)

```bash
bunx playwright test
# or
make test-ui
```

## Documentation

- [Architecture](ARCHITECTURE.md) — System design and components
- [API Reference](API.md) — REST API specification
- Web API quick reference: `/api.html` on any running openByte server
- [Deployment Guide](DEPLOYMENT.md) — Production deployment
- [Performance Guide](PERFORMANCE.md) — Profiling, load testing, perf checks

## Project Structure

```
cmd/
  openbyte/   # Unified server/client entry point
  client/     # Client implementation
  server/     # Server implementation
internal/
  api/        # REST API + HTTP speed test handlers
  config/     # Configuration
  metrics/    # Metrics aggregation + latency histogram
  results/    # SQLite results store
pkg/
  types/      # Shared types
docker/       # Docker + Compose configurations
web/          # Web UI (embedded in binary)
  fonts/      # Self-hosted font files
test/         # Unit, integration, E2E tests
```

## License

MIT License — see [LICENSE](LICENSE)

## Acknowledgments

Inspired by [Breitbandmessung](https://github.com/breitbandmessung) from the German Federal Network Agency (BEREC Net Neutrality Reference Measurement System).
