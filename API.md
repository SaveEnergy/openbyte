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
  "protocol": "tcp" | "udp" | "quic",
  "direction": "download" | "upload" | "bidirectional",
  "duration": 30,
  "streams": 4,
  "packet_size": 1500,
  "mode": "client" | "proxy"
}
```

**Parameters:**
- `protocol` (required): "tcp", "udp", or "quic"
- `direction` (required): "download", "upload", or "bidirectional"
- `duration` (optional): Test duration in seconds (default: 30, min: 1, max: 300)
- `streams` (optional): Number of parallel connections (default: 4, min: 1, max: 16)
- `packet_size` (optional): Packet size in bytes (default: 1500, min: 64, max: 9000)
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
  "test_server_quic": "127.0.0.1:8083",
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
    "packet_size": 1500
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

**GET** `/health`

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
- `duration` (optional): Test duration in seconds (default: 10, max: 60)
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

Latency measurement endpoint.

**Response:**
```json
{
  "pong": true,
  "timestamp": 1704456789123,
  "client_ip": "203.0.113.10",
  "ipv6": false
}
```

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

### 13. WebSocket Stream (Legacy)

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

### Client Mode (CLI)

For accurate network measurement, the CLI uses client mode:

1. Client requests test with `mode: "client"`
2. Server returns test server addresses (`test_server_tcp`, `test_server_udp`)
3. Client connects directly to test server
4. Client performs data transfer and measures throughput locally
5. Client reports results via `/complete` endpoint

```
CLI Client                              Server
    │                                     │
    │ POST /stream/start (mode: client)   │
    ├────────────────────────────────────►│
    │◄── {test_server_tcp: "1.2.3.4:8081"}│
    │                                     │
    │ TCP Connect to 1.2.3.4:8081         │
    │═════════════════════════════════════│
    │                                     │
    │ Data transfer (actual network)      │
    │◄════════════════════════════════════│
    │                                     │
    │ POST /stream/{id}/complete          │
    ├────────────────────────────────────►│
```

### Proxy Mode (Web)

For browser compatibility, the web UI uses proxy mode:

1. Client requests test with `mode: "proxy"` (or omit)
2. Server performs test internally
3. Server streams metrics via WebSocket
4. Client displays results

```
Web Browser                             Server
    │                                     │
    │ POST /stream/start (mode: proxy)    │
    ├────────────────────────────────────►│
    │◄──── {websocket_url: "..."}         │
    │                                     │
    │ WebSocket connect                   │
    │═════════════════════════════════════│
    │                                     │
    │◄════ Metrics (server measures)      │
    │◄════ Complete                       │
```

## Rate Limiting

**Limits:**
- Per IP: 100 requests per minute (default `RATE_LIMIT_PER_IP`)
- Per IP: 10 active tests max (default `MAX_CONCURRENT_PER_IP`)
- Global: 1000 requests per minute (default `GLOBAL_RATE_LIMIT`)

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

