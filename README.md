<h1 align="center" style="font-size:3.4rem; line-height:1.05; letter-spacing:0.01em; margin:0.15em 0;">open<span style="color:#00d4aa;">Byte</span></h1>

<p align="center">
  <a href="https://github.com/SaveEnergy/openbyte/actions/workflows/ci.yml">
    <img src="https://github.com/SaveEnergy/openbyte/actions/workflows/ci.yml/badge.svg" alt="CI">
  </a>
  <a href="https://github.com/SaveEnergy/openbyte/releases">
    <img src="https://img.shields.io/github/v/release/SaveEnergy/openbyte?sort=semver" alt="Latest Release">
  </a>
  <a href="https://github.com/SaveEnergy/openbyte/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/SaveEnergy/openbyte" alt="License">
  </a>
  <a href="https://go.dev/">
    <img src="https://img.shields.io/github/go-mod/go-version/SaveEnergy/openbyte" alt="Go Version">
  </a>
</p>

High-performance network speed test server capable of 25 Gbit/s sustained throughput. TCP/UDP protocols, real-time metrics, multi-connection testing, BEREC-compliant measurement methodology.

## Quick Start

### Build & Run

```bash
make build
./bin/openbyte server

# With server identity
SERVER_ID=nyc-1 SERVER_NAME="New York" ./bin/openbyte server

# With server flags (flags override env values when set)
./bin/openbyte server --server-name "New York" --public-host speedtest.example.com
```

### CLI Client

```bash
./bin/openbyte client -d download -t 30        # 30-second download test
./bin/openbyte client -S nyc                   # Use configured server
./bin/openbyte client speedtest.example.com    # Use remote server
./bin/openbyte client --servers                # List servers
./bin/openbyte client -p http -d download      # HTTP streaming download
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

Open `http://localhost:8080` — minimal fast.com-inspired UI with real-time speed visualization.

## Features

- **Protocols**: TCP, UDP, HTTP streaming
- **Test Types**: Download, Upload, Bidirectional
- **Metrics**: Throughput, Latency (P50/P95/P99), Jitter, Packet Loss
- **RTT**: Baseline and during-test round-trip time measurement
- **Network Info**: Client IP, IPv6 detection, NAT, MTU
- **Streaming**: WebSocket real-time metrics
- **Multi-stream**: 1-64 parallel connections
- **Multi-server**: Deploy globally, test against nearest server
- **Output**: JSON, plain text, interactive CLI

## Measurement Methodology

Uses BEREC-compliant measurement practices:

- Dynamic warm-up with throughput stabilization detection (web UI); fixed warm-up via `--warmup` (CLI)
- Baseline RTT measurement (10 samples before test)
- Metrics reset after warm-up for accurate results
- Statistical reporting with P50, P95, P99 percentiles

## Configuration

### Server Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | HTTP API port |
| `TCP_TEST_PORT` | 8081 | TCP test data port |
| `UDP_TEST_PORT` | 8082 | UDP test data port |
| `SERVER_ID` | hostname | Unique server identifier |
| `SERVER_NAME` | OpenByte Server | Human-readable name |
| `SERVER_LOCATION` | — | Geographic location |
| `SERVER_REGION` | — | Cloud region (optional) |
| `PUBLIC_HOST` | — | Public hostname/IP |
| `CAPACITY_GBPS` | 25 | Server link capacity; HTTP concurrency limits auto-scale from this |
| `RATE_LIMIT_PER_IP` | 100 | Rate limit per IP per minute |
| `GLOBAL_RATE_LIMIT` | 1000 | Global rate limit per minute |
| `TRUST_PROXY_HEADERS` | false | Trust proxy headers for client IP |
| `TRUSTED_PROXY_CIDRS` | — | Comma-separated trusted proxy CIDRs |
| `ALLOWED_ORIGINS` | `*` | Comma-separated CORS/WS allowed origins |
| `WEB_ROOT` | *(embedded)* | Override path to static web assets (for development) |
| `MAX_CONCURRENT_TESTS` | 10 | Maximum simultaneous tests |
| `MAX_STREAMS` | 32 | Maximum parallel streams per test (1-64) |
| `MAX_TEST_DURATION` | `300s` | Maximum test duration (Go duration format) |
| `DATA_DIR` | `./data` | Path to SQLite database directory |
| `MAX_STORED_RESULTS` | 10000 | Maximum stored test results (older results auto-purged) |
| `BIND_ADDRESS` | `0.0.0.0` | Address to bind listeners |
| `PPROF_ENABLED` | false | Enable pprof profiling server |
| `PPROF_ADDR` | `127.0.0.1:6060` | pprof server listen address |
| `PERF_STATS_INTERVAL` | — | Log runtime stats at this interval (e.g. `10s`) |

Notes:
- For reverse proxy deployments, set `TRUST_PROXY_HEADERS=true` and `TRUSTED_PROXY_CIDRS` to the proxy IP ranges.
- Default CORS allows all origins; set `ALLOWED_ORIGINS` to restrict (supports `*` and `*.example.com`).
- If running behind a reverse proxy, increase max request body size (e.g. 35MB) and disable request buffering for `/api/v1/upload` to avoid upload failures or inflated results.
- Server command supports flags for deployment (`openbyte server --help`). If both env var and flag are set, the flag wins.

### Deployment With Server Flags

```bash
# docker run
docker run --rm -p 8080:8080 -p 8081:8081 -p 8082:8082 -p 8082:8082/udp \
  ghcr.io/saveenergy/openbyte:latest \
  server --server-name "My Box" --public-host speed.example.com

# docker compose service command override
# command: ["server", "--server-name=My Box", "--public-host=speed.example.com"]
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
default_server: nyc
servers:
  nyc:
    url: https://speedtest-nyc.example.com
    name: "New York"
  ams:
    url: https://speedtest-ams.example.com
    name: "Amsterdam"
protocol: http
chunk_size: 1048576
```

## Testing

### UI E2E (Playwright)

```bash
bunx playwright test
# or
make test-ui
```

## Multi-Server Deployment

### Registry Service (Optional)

Run a central registry for automatic server discovery:

```bash
# Start registry service
REGISTRY_MODE=true ./bin/openbyte server

# Servers register with registry
REGISTRY_ENABLED=true REGISTRY_URL=http://registry:8080 ./bin/openbyte server
```

Registry API:
- `GET /api/v1/registry/servers` — List all servers
- `GET /api/v1/registry/servers?healthy=true` — List healthy servers
- `POST /api/v1/registry/servers` — Register server
- `DELETE /api/v1/registry/servers/{id}` — Deregister

## Documentation

- [Architecture](ARCHITECTURE.md) — System design and components
- [API Reference](API.md) — REST API specification
- [Deployment Guide](DEPLOYMENT.md) — Production deployment
- [Performance Guide](PERFORMANCE.md) — Profiling, load testing, perf checks

## Project Structure

```
cmd/
  openbyte/   # Unified server/client entry point
  client/     # Client implementation
  server/     # Server implementation
  loadtest/   # Load generator
internal/
  api/        # REST API + HTTP speed test handlers
  config/     # Configuration
  metrics/    # Metrics collection + latency histogram
  registry/   # Server registry
  results/    # SQLite results store
  stream/     # TCP/UDP test engine
  websocket/  # Real-time metrics streaming
pkg/
  types/      # Shared types
  errors/     # Error definitions
docker/       # Docker + Compose configurations
web/          # Web UI (embedded in binary)
  fonts/      # Self-hosted font files
test/         # Unit, integration, E2E tests
```

## License

MIT License — see [LICENSE](LICENSE)

## Acknowledgments

Inspired by [Breitbandmessung](https://github.com/breitbandmessung) from the German Federal Network Agency (BEREC Net Neutrality Reference Measurement System).
