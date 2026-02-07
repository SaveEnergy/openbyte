# Architecture

High-performance network speed test server. Target: 25 Gbit/s sustained throughput. Multi-protocol (TCP/UDP/HTTP streaming), real-time metrics, concurrent tests.

## System Overview

```
┌─────────────┐
│   Client    │ (Web/CLI)
└──────┬──────┘
       │ HTTP/WebSocket
       ▼
┌─────────────────────────────────────┐
│         API Gateway                 │
│  (REST + WebSocket + HTTP Streams)  │
└──────┬──────────────────┬───────────┘
       │                  │
       ▼                  ▼
┌──────────────┐   ┌──────────────┐
│ Test Manager │   │ Metrics      │
│(Orchestrator)│   │ Collector    │
└──────┬───────┘   └──────┬───────┘
       │                  │
       ▼                  │
┌─────────────────────────┴──────────┐
│      Test Engine (Core)            │
│  ┌──────────┐  ┌──────────┐       │
│  │ TCP Test │  │ UDP Test │       │
│  │ Handler  │  │ Handler  │       │
│  └──────────┘  └──────────┘       │
└────────────────────────────────────┘
       │
       ▼
┌─────────────────────┐
│    Network Stack    │
│   (Go stdlib net)   │
└─────────────────────┘
```

## Components

### API Gateway

HTTP REST API, WebSocket server for real-time metrics, and HTTP streaming endpoints for browser-based speed tests.

**Endpoints:**
```
POST   /api/v1/stream/start           # Start TCP/UDP test
GET    /api/v1/stream/{id}/status     # Test status
GET    /api/v1/stream/{id}/results    # Final results
POST   /api/v1/stream/{id}/cancel     # Cancel test
POST   /api/v1/stream/{id}/metrics    # Client reports metrics
POST   /api/v1/stream/{id}/complete   # Client reports completion
WS     /api/v1/stream/{id}/stream     # Real-time metrics
GET    /api/v1/servers                # List servers
GET    /api/v1/version                # Build version
GET    /api/v1/download               # HTTP streaming download
POST   /api/v1/upload                 # HTTP streaming upload
GET    /api/v1/ping                   # Latency + IP detection
POST   /api/v1/results                # Save test result
GET    /api/v1/results/{id}           # Get saved result
GET    /health                        # Health check
```

**Implementation:**
- `gorilla/mux` for routing
- `gorilla/websocket` for real-time streaming
- Token bucket rate limiting (per-IP + global)
- CORS middleware with wildcard pattern support
- Security headers (CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy)
- Self-hosted fonts (embedded in binary via `//go:embed`)

### Test Manager

Orchestrates test lifecycle: create, start, monitor, complete, cleanup.

**State Machine:**
```
pending → running → completed
              ↓
            failed
```

**Responsibilities:**
- Test state tracking (in-memory map, atomic active count)
- Resource allocation and concurrency limits
- Duration cap enforcement
- Periodic cleanup of stale/expired tests
- Metrics broadcast to WebSocket clients

### Test Engine

Core data plane handling TCP/UDP network I/O.

**Architecture:**
- Goroutine per stream (bounded by `MaxConcurrentTests`)
- Atomic counters for bytes/packets
- Buffer pooling (`sync.Pool` for receive buffers)
- Fixed-bucket latency histogram (1ms buckets, `sync.RWMutex` protected)

**TCP Handler:**
- Download: writes pre-generated random data in chunks
- Upload: reads into pooled buffers, tracks bytes
- Bidirectional: concurrent read/write goroutines with latency recording
- Connection limits and duration caps enforced

**UDP Handler:**
- Sequence numbers for packet loss detection
- Sender concurrency limits with WaitGroup tracking
- Per-packet timestamps for jitter measurement

### HTTP Streaming (Web UI)

Browser-based speed tests using standard HTTP:

- **Download** (`GET /api/v1/download`): Streams random data as `application/octet-stream` for configurable duration
- **Upload** (`POST /api/v1/upload`): Accepts binary body, returns bytes/duration/throughput
- **Ping** (`GET /api/v1/ping`): Returns server timestamp, client IP, IPv6 flag

The web UI runs multiple concurrent fetch streams with dynamic warm-up detection and EWMA-smoothed live speed display.

### Metrics Collector

Per-stream metric collection using atomic operations and fixed-bucket histograms.

**Metrics:**
- Throughput: `(bytes × 8) / seconds / 1,000,000` Mbps
- Latency: Fixed-bucket histogram (1ms resolution, O(1) percentile calculation)
- Jitter: Mean consecutive difference (RFC 3550)
- Packet loss: `(sent - received) / sent × 100` (clamped ≥ 0)

### Results Store

SQLite-backed persistent storage for shareable test results.

- WAL mode with busy timeout for concurrent access
- 8-character alphanumeric IDs
- Configurable retention and max stored results
- Background cleanup goroutine

## Data Flow

### Web UI Speed Test

```
Browser                                Server
   │                                     │
   │  GET /api/v1/ping (×24)             │
   ├────────────────────────────────────►│  Latency measurement
   │                                     │
   │  GET /api/v1/download (×N streams)  │
   ├────────────────────────────────────►│  Download test
   │◄════════════════════════════════════│  (streaming response)
   │                                     │
   │  POST /api/v1/upload (×N streams)   │
   ├════════════════════════════════════►│  Upload test
   │                                     │
   │  POST /api/v1/results               │
   ├────────────────────────────────────►│  Save & share result
```

### CLI Speed Test (TCP/UDP)

```
CLI Client                              Server
    │                                     │
    │ POST /api/v1/stream/start           │
    ├────────────────────────────────────►│
    │◄── {test_server_tcp: "1.2.3.4:8081"}│
    │                                     │
    │ TCP/UDP connect to test port        │
    │═════════════════════════════════════│
    │ Data transfer (measured locally)    │
    │                                     │
    │ POST /api/v1/stream/{id}/complete   │
    ├────────────────────────────────────►│
```

### CLI Speed Test (HTTP)

```
CLI Client                              Server
    │                                     │
    │ GET /api/v1/download                │
    │◄════════════════════════════════════│  HTTP streaming
    │                                     │
    │ POST /api/v1/upload                 │
    │════════════════════════════════════►│  HTTP upload
    │                                     │
    │ GET /api/v1/ping (×N)               │
    ├────────────────────────────────────►│  Latency
```

## Performance

### Memory Management

- Receive buffer pooling (`sync.Pool`, 256KB buffers)
- Pre-generated 1MB random data block (shared across downloads)
- Fixed-bucket histogram avoids per-sample allocation

### Concurrency

- Goroutine per stream
- Atomic counters for hot-path metrics (bytes, packets)
- `sync.RWMutex` for histogram (writers exclusive, readers concurrent)
- `sync.Once` for idempotent Stop() on manager/registry
- `sync.WaitGroup` for graceful shutdown of background goroutines
- Context-based cancellation throughout

## Scalability

### Horizontal

- Load balancer distributes tests
- Each server handles subset of tests
- Optional registry service for automatic discovery

### Vertical

- `MAX_CONCURRENT_TESTS` limit
- `MAX_STREAMS` per test (1-64)
- `MAX_TEST_DURATION` cap
- HTTP concurrency auto-scales with `CAPACITY_GBPS`
- Per-IP and global rate limiting

## Security

### Rate Limiting

Token bucket with fractional remainder preservation:
- Per-IP: configurable requests per minute (default 100)
- Per-IP: concurrent test limit
- Global: server-wide rate limit (default 1000/min)

### Headers

- `Content-Security-Policy`: `script-src 'self'`, `font-src 'self'`, `connect-src *`
- `X-Content-Type-Options`: `nosniff`
- `X-Frame-Options`: `DENY`
- `Referrer-Policy`: `strict-origin-when-cross-origin`

### Input Validation

- Duration: 1-300 seconds
- Streams: 1-64
- Packet size: 64-9000 bytes
- Chunk size: 65536-4194304 bytes
- Stream IDs: UUID format enforced at routing layer
- JSON body size limits on all POST endpoints
- Port collision detection (HTTP vs TCP/UDP)

### CORS

- Wildcard pattern support (`*.example.com`) with dot-boundary enforcement
- WebSocket origins validated against same allowlist
- Configurable via `ALLOWED_ORIGINS`

## Deployment

### Container

See `docker/Dockerfile` for the multi-stage build. The binary embeds all web assets (HTML, CSS, JS, fonts) via `//go:embed`.

```
EXPOSE 8080 8081 8082/tcp 8082/udp
```

### Systemd

```ini
[Unit]
Description=openByte Speed Test Server
After=network.target

[Service]
Type=simple
User=openbyte
ExecStart=/opt/openbyte/openbyte server
Restart=always

[Install]
WantedBy=multi-user.target
```

See [Deployment Guide](DEPLOYMENT.md) for full production setup.

## Monitoring

### Health Check

`GET /health` returns `{"status": "ok"}`.

### Runtime Stats

Optional periodic logging of goroutine count, heap usage, GC stats:

```
PERF_STATS_INTERVAL=5s ./bin/openbyte server
```

### pprof

```
PPROF_ENABLED=true PPROF_ADDR=127.0.0.1:6060 ./bin/openbyte server
```

### Logging

Structured JSON logging with levels: INFO, WARN, ERROR.
