# Network Speed Test Server - API Specification

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Public API by default (no auth required). Registry endpoints (`/api/v1/registry/*`) require a Bearer token when `REGISTRY_API_KEY` is set.

## Endpoints

### 1. Start Test

**POST** `/stream/start`

Initiate new speed test.

**Request Body:**
```json
{
  "protocol": "tcp" | "udp",
  "direction": "download" | "upload" | "bidirectional",
  "duration": 30,
  "streams": 4,
  "packet_size": 1400,
  "mode": "client" | "proxy"
}
```

**Parameters:**
- `protocol` (required): "tcp" or "udp"
- `direction` (required): "download", "upload", or "bidirectional"
- `duration` (optional): Test duration in seconds (default: 30, min: 1, max: 300)
- `streams` (optional): Number of parallel connections (default: 4, min: 1, max: 64)
- `packet_size` (optional): Packet size in bytes (default: 1400, min: 64, max: 9000). Default avoids IP fragmentation on standard 1500-byte MTU links (1400 + 28 IPv4/UDP headers = 1428 < 1500).
- `mode` (optional): Testing mode (default: "proxy")
  - `"client"`: Client-side testing (CLI) - client connects to test server directly
  - `"proxy"`: Server-side testing (Web) - server performs test on behalf of client

**Response (mode: client):**
```json
{
  "stream_id": "550e8400-e29b-41d4-a716-446655440000",
  "websocket_url": "/api/v1/stream/550e8400-e29b-41d4-a716-446655440000/stream",
  "test_server_tcp": "127.0.0.1:8081",
  "test_server_udp": "127.0.0.1:8082",
  "status": "running",
  "mode": "client"
}
```

**Response (mode: proxy):**
```json
{
  "stream_id": "550e8400-e29b-41d4-a716-446655440000",
  "websocket_url": "/api/v1/stream/550e8400-e29b-41d4-a716-446655440000/stream",
  "status": "running",
  "mode": "proxy"
}
```

**Status Codes:**
- `201 Created`: Test started successfully
- `400 Bad Request`: Invalid parameters
- `429 Too Many Requests`: Rate limit exceeded
- `503 Service Unavailable`: Server at capacity

**Example (CLI mode):**
```bash
curl -X POST http://localhost:8080/api/v1/stream/start \
  -H "Content-Type: application/json" \
  -d '{
    "protocol": "tcp",
    "direction": "download",
    "duration": 30,
    "streams": 4,
    "mode": "client"
  }'
```

### 2. Get Test Status

**GET** `/stream/{stream_id}/status`

Get current test status and progress.

**Response:**
```json
{
  "stream_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "running" | "completed" | "failed" | "pending",
  "progress": 45.5,
  "elapsed_seconds": 13.5,
  "remaining_seconds": 16.5,
  "metrics": {
    "throughput_mbps": 24500.5,
    "latency_ms": {
      "min_ms": 0.1,
      "max_ms": 2.5,
      "avg_ms": 0.5,
      "p50_ms": 0.4,
      "p95_ms": 1.2,
      "p99_ms": 2.0
    },
    "jitter_ms": 0.15,
    "packet_loss_percent": 0.01
  }
}
```

**Status Codes:**
- `200 OK`: Status retrieved
- `404 Not Found`: Test not found

### 3. Get Test Results

**GET** `/stream/{stream_id}/results`

Get final test results (after completion).

**Response:**
```json
{
  "stream_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "config": {
    "protocol": "tcp",
    "direction": "download",
    "duration": 30,
    "streams": 4,
    "packet_size": 1400
  },
  "results": {
    "throughput_mbps": 25000.0,
    "throughput_avg_mbps": 24850.5,
    "latency_ms": {
      "min_ms": 0.1,
      "max_ms": 2.5,
      "avg_ms": 0.5,
      "p50_ms": 0.4,
      "p95_ms": 1.2,
      "p99_ms": 2.0
    },
    "jitter_ms": 0.15,
    "packet_loss_percent": 0.01,
    "bytes_transferred": 93750000000,
    "packets_sent": 62500000,
    "packets_received": 62493750
  },
  "start_time": "2026-01-05T10:00:00Z",
  "end_time": "2026-01-05T10:00:30Z",
  "duration_seconds": 30.0
}
```

**Status Codes:**
- `200 OK`: Results retrieved
- `404 Not Found`: Test not found
- `202 Accepted`: Test still running

### 4. Cancel Test

**POST** `/stream/{stream_id}/cancel`

Cancel a running test.

**Response:**
```json
{
  "status": "cancelled"
}
```

**Status Codes:**
- `200 OK`: Test cancelled
- `404 Not Found`: Test not found

### 5. Report Metrics (Client Mode)

**POST** `/stream/{stream_id}/metrics`

Client reports metrics during test (client mode only).

**Request Body:**
```json
{
  "throughput_mbps": 24500.5,
  "bytes_transferred": 1073741824,
  "latency_ms": {
    "min_ms": 0.1,
    "max_ms": 2.5,
    "avg_ms": 0.5
  },
  "jitter_ms": 0.15
}
```

**Response:**
```json
{
  "status": "accepted"
}
```

**Status Codes:**
- `202 Accepted`: Metrics received
- `404 Not Found`: Test not found

### 6. Complete Test (Client Mode)

**POST** `/stream/{stream_id}/complete`

Client reports test completion with final metrics (client mode only).

**Request Body:**
```json
{
  "status": "completed",
  "metrics": {
    "throughput_mbps": 25000.0,
    "throughput_avg_mbps": 24850.5,
    "bytes_transferred": 93750000000,
    "latency_ms": {
      "min_ms": 0.1,
      "max_ms": 2.5,
      "avg_ms": 0.5,
      "p50_ms": 0.4,
      "p95_ms": 1.2,
      "p99_ms": 2.0
    },
    "jitter_ms": 0.15
  }
}
```

**Response:**
```json
{
  "status": "ok"
}
```

**Status Codes:**
- `200 OK`: Completion recorded
- `404 Not Found`: Test not found

### 7. List Servers

**GET** `/servers`

Get available test servers with full metadata.

**Response:**
```json
{
  "servers": [
    {
      "id": "nyc-1",
      "name": "New York",
      "location": "US-East",
      "region": "us-east-1",
      "host": "speedtest-nyc.example.com",
      "tcp_port": 8081,
      "udp_port": 8082,
      "api_endpoint": "https://speedtest-nyc.example.com:8080",
      "health": "healthy",
      "capacity_gbps": 25,
      "active_tests": 3,
      "max_tests": 10
    }
  ]
}
```

**Fields:**
- `id`: Unique server identifier
- `name`: Human-readable server name
- `location`: Geographic location
- `region`: Cloud region (optional)
- `host`: Server hostname/IP
- `tcp_port`: TCP test port (default: 8081)
- `udp_port`: UDP test port (default: 8082)
- `api_endpoint`: Full API endpoint URL
- `health`: Server health status ("healthy", "degraded", "offline")
- `capacity_gbps`: Maximum capacity in Gbps
- `active_tests`: Current running tests
- `max_tests`: Maximum concurrent tests

**Status Codes:**
- `200 OK`: Servers listed

### 8. Health Check

**GET** `/health` *(root path, not under `/api/v1`)*

Server health status.

**Response:**
```json
{
  "status": "ok"
}
```

**Status Codes:**
- `200 OK`: Server healthy

### 9. Download Test

**GET** `/download`

Stream random data to client for download speed measurement.

**Query Parameters:**
- `duration` (optional): Test duration in seconds (default: 10, max: configurable via `MAX_TEST_DURATION`, default 300)
- `chunk` (optional): Chunk size in bytes (default: 1048576, range: 65536-4194304)

**Response:**
- Content-Type: `application/octet-stream`
- Streams random binary data for the specified duration

**Example:**
```bash
curl -o /dev/null http://localhost:8080/api/v1/download?duration=5
```

### 10. Upload Test

**POST** `/upload`

Receive data from client for upload speed measurement.

**Request:**
- Content-Type: `application/octet-stream`
- Body: Binary data to upload

**Response:**
```json
{
  "bytes": 1048576,
  "duration_ms": 100,
  "throughput_mbps": 83.89
}
```

**Status Codes:**
- `200 OK`: Upload received
- `503 Service Unavailable`: Too many concurrent uploads

### 11. Ping

**GET** `/ping`

Latency measurement and network detection endpoint. Used for latency sampling (24 pings, 2 warm-up discarded) and client IP / IPv6 detection.

**Response:**
```json
{
  "pong": true,
  "timestamp": 1704456789123,
  "client_ip": "203.0.113.10",
  "ipv6": false
}
```

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `pong` | bool | Always `true` |
| `timestamp` | int | Server timestamp (Unix milliseconds) |
| `client_ip` | string | Client IP as seen by the server (respects `X-Forwarded-For` when `TRUST_PROXY_HEADERS=true`) |
| `ipv6` | bool | `true` if `client_ip` is an IPv6 address |

**IPv6 Detection:**

The web UI uses this endpoint in two ways:

1. **Main connection probe** — calls `/ping` on page load and during latency measurement. Reports the address family of the actual browser connection.
2. **IPv6 capability probe** — calls `/ping` on the `v6.` subdomain (AAAA-only DNS). If the response has `ipv6: true`, the client has IPv6 connectivity even if the browser chose IPv4 for the main connection.

See [Deployment Guide](DEPLOYMENT.md#ipv4ipv6-detection) for DNS setup.

### 12. Version

**GET** `/version`

Build/version information.

**Response:**
```json
{
  "version": "v1.2.3"
}
```

**Status Codes:**
- `200 OK`: Version returned

### 13. WebSocket Stream

**WS** `/stream/{stream_id}/stream`

Real-time metrics streaming.

**Connection:**
```
ws://localhost:8080/api/v1/stream/{stream_id}/stream
```

**Messages (Server → Client):**

**Progress Update:**
```json
{
  "type": "progress",
  "progress": 45.5,
  "elapsed_seconds": 13.5,
  "remaining_seconds": 16.5
}
```

**Metrics Update:**
```json
{
  "type": "metrics",
  "timestamp": "2026-01-05T10:00:13.500Z",
  "metrics": {
    "throughput_mbps": 24500.5,
    "latency_ms": {
      "min_ms": 0.1,
      "max_ms": 2.5,
      "avg_ms": 0.5,
      "p50_ms": 0.4,
      "p95_ms": 1.2,
      "p99_ms": 2.0
    },
    "jitter_ms": 0.15,
    "packet_loss_percent": 0.01
  }
}
```

**Test Complete:**
```json
{
  "type": "complete",
  "results": {
    "stream_id": "...",
    "status": "completed",
    "config": {...},
    "results": {...}
  }
}
```

**Test Failed:**
```json
{
  "type": "error",
  "error": "Connection timeout",
  "message": "Test failed: connection to client lost"
}
```

**Update Frequency:**
- Progress: Every 1 second
- Metrics: Every 1 second

## Testing Modes

### HTTP Streaming (Web UI)

The web UI uses standard HTTP endpoints for speed tests:

1. Latency: 24 pings to `/ping` (2 warm-up discarded)
2. Download: Concurrent `GET /download` streams with dynamic warm-up detection
3. Upload: Concurrent `POST /upload` requests
4. Results saved via `POST /results`

```
Web Browser                              Server
    │                                     │
    │  GET /ping (×24)                    │
    ├────────────────────────────────────►│  Latency
    │                                     │
    │  GET /download (×N streams)         │
    │◄════════════════════════════════════│  Download
    │                                     │
    │  POST /upload (×N streams)          │
    │════════════════════════════════════►│  Upload
    │                                     │
    │  POST /results                      │
    ├────────────────────────────────────►│  Save
```

### TCP/UDP Client Mode (CLI)

The CLI can use TCP/UDP for direct data-plane measurement:

1. Client requests test via `POST /stream/start` with `mode: "client"`
2. Server returns test server addresses (`test_server_tcp`, `test_server_udp`)
3. Client connects directly to test port
4. Client performs data transfer and measures throughput locally
5. Client reports results via `POST /stream/{id}/complete`

```
CLI Client                              Server
    │                                     │
    │ POST /stream/start (mode: client)   │
    ├────────────────────────────────────►│
    │◄── {test_server_tcp: "1.2.3.4:8081"}│
    │                                     │
    │ TCP/UDP Connect to test port        │
    │═════════════════════════════════════│
    │ Data transfer (measured locally)    │
    │                                     │
    │ POST /stream/{id}/complete          │
    ├────────────────────────────────────►│
```

### HTTP Client Mode (CLI)

The CLI also supports HTTP streaming (same endpoints as the web UI):

```
CLI Client                              Server
    │                                     │
    │ GET /download                       │
    │◄════════════════════════════════════│  HTTP streaming
    │                                     │
    │ POST /upload                        │
    │════════════════════════════════════►│  Upload
    │                                     │
    │ GET /ping (×N)                      │
    ├────────────────────────────────────►│  Latency
```

## Saved Results

### Save Result

`POST /api/v1/results`

Save a completed test result. Returns a short ID and URL for sharing.

**Request:**
```json
{
  "download_mbps": 500.5,
  "upload_mbps": 100.2,
  "latency_ms": 8.1,
  "jitter_ms": 0.5,
  "loaded_latency_ms": 15.3,
  "bufferbloat_grade": "B",
  "ipv4": "203.0.113.1",
  "ipv6": "2001:db8::1",
  "server_name": "Production Server"
}
```

**Response (201):**
```json
{
  "id": "a3kf8x2b",
  "url": "/results/a3kf8x2b"
}
```

**Validation:**
- All numeric fields must be >= 0
- Speed values capped at 100,000 Mbps; latency values at 60,000 ms
- String fields have length limits (server_name: 200, IPs: 45, grade: 5)

### Get Result

`GET /api/v1/results/{id}`

Retrieve a saved test result by its 8-character alphanumeric ID.

**Response (200):**
```json
{
  "id": "a3kf8x2b",
  "download_mbps": 500.5,
  "upload_mbps": 100.2,
  "latency_ms": 8.1,
  "jitter_ms": 0.5,
  "loaded_latency_ms": 15.3,
  "bufferbloat_grade": "B",
  "ipv4": "203.0.113.1",
  "ipv6": "2001:db8::1",
  "server_name": "Production Server",
  "created_at": "2026-02-05T12:00:00Z"
}
```

Returns 404 if the result is not found or has expired (90-day retention).

### Result Page

`GET /results/{id}` serves a standalone HTML page that fetches and renders the saved result.

## Registry API (Optional)

Server registry for multi-server deployments. Enable with `REGISTRY_MODE=true`. Protected endpoints require `Authorization: Bearer <REGISTRY_API_KEY>` when `REGISTRY_API_KEY` is set.

### List Servers

**GET** `/api/v1/registry/servers`

**Query Parameters:**
- `healthy` (optional): `"true"` to return only healthy servers

**Response (200):**
```json
{
  "servers": [
    {
      "id": "nyc-1",
      "name": "New York",
      "location": "US-East",
      "host": "speedtest-nyc.example.com",
      "tcp_port": 8081,
      "udp_port": 8082,
      "api_endpoint": "https://speedtest-nyc.example.com:8080",
      "health": "healthy",
      "capacity_gbps": 25,
      "active_tests": 3,
      "max_tests": 10,
      "last_heartbeat": "2026-02-07T12:00:00Z"
    }
  ],
  "count": 1
}
```

### Register Server

**POST** `/api/v1/registry/servers` *(auth required)*

**Request Body:**
```json
{
  "id": "nyc-1",
  "name": "New York",
  "location": "US-East",
  "host": "speedtest-nyc.example.com",
  "tcp_port": 8081,
  "udp_port": 8082,
  "capacity_gbps": 25
}
```

**Response (201):**
```json
{
  "status": "registered",
  "server_id": "nyc-1"
}
```

### Get Server

**GET** `/api/v1/registry/servers/{id}`

**Response (200):** Same as individual entry from list endpoint.

**Status Codes:** `200` OK, `404` Not Found.

### Update Server (Heartbeat)

**PUT** `/api/v1/registry/servers/{id}` *(auth required)*

Same body as register. Used for periodic heartbeats with updated `active_tests`.

**Response (200):**
```json
{
  "status": "updated",
  "server_id": "nyc-1"
}
```

### Deregister Server

**DELETE** `/api/v1/registry/servers/{id}` *(auth required)*

**Response (200):**
```json
{
  "status": "deregistered",
  "server_id": "nyc-1"
}
```

### Registry Health

**GET** `/api/v1/registry/health`

**Response (200):**
```json
{
  "status": "healthy",
  "servers": 5
}
```

## Rate Limiting

**Limits:**
- Per IP: 100 requests per minute (default `RATE_LIMIT_PER_IP`)
- Global: 1000 requests per minute (default `GLOBAL_RATE_LIMIT`)
- HTTP download/upload concurrency auto-scales with `CAPACITY_GBPS`

## CORS and WebSocket Origins

- CORS allowlist is controlled via `ALLOWED_ORIGINS`.
- WebSocket origins are validated against the same allowlist.
- If `ALLOWED_ORIGINS` is empty, browser cross-origin requests are blocked (same-origin only).

## Error Responses

**Standard Error Format:**
```json
{
  "error": "Human-readable error message"
}
```

