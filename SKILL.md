# openByte Integration Guide

Use this skill when integrating with the openByte speed test API for network diagnostics, connectivity checks, and throughput measurement.

## When to Use

- Running network speed tests (download/upload throughput)
- Quick connectivity checks with grading (A-F)
- Diagnosing network quality (latency, jitter, bufferbloat)
- Monitoring server health and reachability
- Building network-aware applications or dashboards

## Quick Reference

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Server health check |
| GET | `/api/v1/ping` | Latency measurement + client IP detection |
| GET | `/api/v1/download?duration=N&chunk=SIZE` | Download speed test (streams binary) |
| POST | `/api/v1/upload` | Upload speed test (accepts binary body) |
| POST | `/api/v1/stream/start` | Start managed test session |
| GET | `/api/v1/stream/{id}/status` | Poll test status and live metrics |
| GET | `/api/v1/stream/{id}/results` | Get final test results |
| POST | `/api/v1/stream/{id}/cancel` | Cancel running test |
| GET | `/api/v1/version` | Server version |
| GET | `/api/v1/servers` | List available test servers |
| POST | `/api/v1/results` | Save test results |
| GET | `/api/v1/results/{id}` | Retrieve saved results |

### Authentication

Public API — no auth required. Registry endpoints require Bearer token when `REGISTRY_API_KEY` is set.

### Response Format

All errors return JSON:
```json
{"error": "description"}
```

## MCP Server (for AI Agents)

Start the MCP server over stdio:

```bash
openbyte mcp
```

### Tools

| Tool | Duration | Description |
|------|----------|-------------|
| `connectivity_check` | ~3-5s | Quick latency + burst download/upload. Returns grade A-F. |
| `speed_test` | configurable | Full speed test. Params: `server_url`, `direction`, `duration`. |
| `diagnose` | ~15-20s | Comprehensive: 10 latency samples, 5s download, 5s upload, full interpretation. |

All tools accept an optional `server_url` parameter (default: `http://localhost:8080`).

### MCP Response Format

Every tool returns JSON with an `interpretation` object:

```json
{
  "status": "ok",
  "latency_ms": 12.5,
  "download_mbps": 450.2,
  "upload_mbps": 95.1,
  "jitter_ms": 1.3,
  "interpretation": {
    "grade": "A",
    "summary": "Excellent connection: 450 Mbps down, 95 Mbps up, 13ms latency",
    "latency_rating": "excellent",
    "speed_rating": "fast",
    "stability_rating": "stable",
    "suitable_for": ["web_browsing", "video_conferencing", "streaming_4k", "gaming", "large_transfers"],
    "concerns": []
  }
}
```

## Go SDK

Import `github.com/saveenergy/openbyte/pkg/client`:

```go
import "github.com/saveenergy/openbyte/pkg/client"
```

### Quick Check (~3-5 seconds)

```go
c := client.New("https://speed.example.com")
result, err := c.Check(ctx)
// result.LatencyMs, result.DownloadMbps, result.UploadMbps
// result.Interpretation.Grade  ("A" through "F")
```

### Full Speed Test

```go
result, err := c.SpeedTest(ctx, client.SpeedTestOptions{
    Direction: "download",  // or "upload"
    Duration:  10,          // seconds (1-300)
})
// result.ThroughputMbps, result.BytesTotal, result.DurationSec
```

### Comprehensive Diagnosis (~15-20 seconds)

```go
result, err := c.Diagnose(ctx)
// result.DownloadMbps, result.UploadMbps, result.LatencyMs, result.JitterMs
// result.Interpretation — full grade + ratings + suitability
```

### Health Check

```go
err := c.Healthy(ctx) // nil if server is reachable and healthy
```

### Options

```go
c := client.New("https://speed.example.com",
    client.WithAPIKey("your-key"),           // optional auth
    client.WithHTTPClient(customHTTPClient), // optional custom client
)
```

## CLI

### Quick Check

```bash
openbyte check                                # localhost
openbyte check https://speed.example.com      # remote server
openbyte check --json                         # JSON output for scripts
openbyte check --json --api-key KEY           # authenticated
```

Exit codes: `0` = healthy (grade A-C), `1` = degraded (grade D-F) or error.

### Full Speed Test

```bash
openbyte client -S https://speed.example.com -d download -t 10
openbyte client -S https://speed.example.com -d upload -t 10 --json
```

## Diagnostic Interpretation

All results include an `interpretation` with:

| Field | Values | Description |
|-------|--------|-------------|
| `grade` | A, B, C, D, F | Overall connection quality |
| `latency_rating` | excellent, good, fair, poor | Based on RTT thresholds |
| `speed_rating` | fast, good, moderate, slow | Based on throughput |
| `stability_rating` | stable, fair, degraded, unstable | Based on jitter + packet loss |
| `suitable_for` | list of workloads | web_browsing, video_conferencing, streaming_4k, gaming, large_transfers |
| `concerns` | list of issues | high_latency, high_jitter, packet_loss, slow_download, slow_upload |

### Grade Thresholds

| Grade | Score Range | Meaning |
|-------|------------|---------|
| A | 11-12 | Excellent |
| B | 9-10 | Good |
| C | 6-8 | Fair |
| D | 3-5 | Poor |
| F | 0-2 | Very poor |

Score = latency_score + speed_score + stability_score (each 0-4).

## Quick Start Examples

### 1. Health Check

```bash
curl -s http://localhost:8080/health
```

```json
{"status":"ok"}
```

### 2. Latency Ping

```bash
curl -s http://localhost:8080/api/v1/ping
```

```json
{"pong":true,"client_ip":"192.168.1.5","ipv6":false}
```

### 3. Download Test (2 seconds)

```bash
curl -s -o /dev/null -w '%{size_download} bytes in %{time_total}s' \
  'http://localhost:8080/api/v1/download?duration=2&chunk=1048576'
```

### 4. Upload Test

```bash
dd if=/dev/zero bs=1M count=4 2>/dev/null | \
  curl -s -X POST http://localhost:8080/api/v1/upload \
  -H 'Content-Type: application/octet-stream' --data-binary @-
```

```json
{"bytes_received":4194304}
```

### 5. Managed Test Session

```bash
# Start test
curl -s -X POST http://localhost:8080/api/v1/stream/start \
  -H 'Content-Type: application/json' \
  -d '{"protocol":"tcp","direction":"download","duration":10,"mode":"proxy"}'

# Poll status (use stream_id from response)
curl -s http://localhost:8080/api/v1/stream/STREAM_ID/status

# Get final results
curl -s http://localhost:8080/api/v1/stream/STREAM_ID/results
```

## Installation

### Binary

```bash
# One-liner install (Linux/macOS)
curl -fsSL https://raw.githubusercontent.com/SaveEnergy/openbyte/main/scripts/install.sh | sh
```

### Go Install

```bash
go install github.com/saveenergy/openbyte/cmd/openbyte@latest
```

### Docker

```bash
docker run -p 8080:8080 -p 8081:8081 -p 8082:8082/udp \
  ghcr.io/saveenergy/openbyte:latest server
```

## OpenAPI Spec

Full OpenAPI 3.1 spec available at `api/openapi.yaml` in the repository.

## Related

- `API.md` — complete endpoint documentation with all parameters
- `ARCHITECTURE.md` — system design and architecture decisions
- `PERFORMANCE.md` — profiling and benchmarking guide
- `DEPLOYMENT.md` — production deployment guide
