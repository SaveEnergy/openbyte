# Architecture

High-performance network speed test server. Target: 25 Gbit/s sustained throughput. Multi-protocol (TCP/UDP), real-time metrics, concurrent tests.

## System Overview

```
┌─────────────┐
│   Client    │ (Web/CLI)
└──────┬──────┘
       │ HTTP/WebSocket
       ▼
┌─────────────────────────────────────┐
│         API Gateway                 │
│  (REST + WebSocket Handler)         │
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
│  ┌──────────┐  ┌──────────┐        │
│  │ TCP Test │  │ UDP Test │        │
│  │ Handler  │  │ Handler  │        │
│  └────┬─────┘  └─────┬────┘        │
│       │              │             │
│  ┌────┴──────────────┴─────┐       │
│  │  Connection Pool        │       │
│  │  (Multi-stream support) │       │
│  └─────────────────────────┘       │
└────────────────────────────────────┘
       │
       ▼
┌─────────────────────┐
│    Network Stack    │
│  (Standard/io_uring)│
└─────────────────────┘
```

## Components

### API Gateway

HTTP REST API and WebSocket server for real-time metrics streaming.

**Endpoints:**
```
POST   /api/v1/stream/start           # Start test
GET    /api/v1/stream/{id}/status     # Test status
GET    /api/v1/stream/{id}/results    # Final results
POST   /api/v1/stream/{id}/cancel     # Cancel test
WS     /api/v1/stream/{id}/stream     # Real-time metrics
GET    /api/v1/servers                # List servers
GET    /api/v1/health                 # Health check
```

**Implementation:**
- `gorilla/mux` for routing
- `gorilla/websocket` for real-time streaming
- Rate limiting middleware (token bucket)
- CORS support for cross-origin requests

### Test Manager

Orchestrates test lifecycle: create, start, monitor, complete, cleanup.

**State Machine:**
```
pending → starting → running → completed
                    ↓
                  failed
```

**Responsibilities:**
- Test state tracking (in-memory map)
- Resource allocation
- Timeout enforcement
- Cleanup of stale tests

### Test Engine

Core data plane handling network I/O and metric collection.

**Architecture:**
- Worker pool per test (goroutines per stream)
- Lock-free metrics (atomic operations)
- Buffer pooling (sync.Pool)
- Zero-copy where possible

**TCP Handler:**
```go
type TCPHandler struct {
    conn      net.Conn
    config    StreamConfig
    metrics   *MetricsCollector
    buffer    []byte  // pre-allocated
}
```

**UDP Handler:**
```go
type UDPHandler struct {
    conn        *net.UDPConn
    config      StreamConfig
    seqNum      uint64  // sequence for loss detection
    sentPackets int64   // atomic
    recvPackets int64   // atomic
}
```

### Metrics Collector

Real-time metric calculation and aggregation.

**Metrics:**
```go
type Metrics struct {
    ThroughputMbps    float64
    LatencyMinMs      float64
    LatencyMaxMs      float64
    LatencyAvgMs      float64
    LatencyP50Ms      float64
    LatencyP95Ms      float64
    LatencyP99Ms      float64
    JitterMs          float64
    PacketLossPercent float64
    BytesTransferred  int64
}
```

**Calculations:**
- Throughput: `(bytes * 8) / seconds / 1_000_000` Mbps
- Latency: RTT measurement via TCP ACK or UDP echo
- Jitter: Variance of latency samples
- Packet loss: `(sent - received) / sent * 100`

### Connection Pool

Multi-stream connection management for parallel testing.

```go
type ConnectionPool struct {
    connections []net.Conn
    config      StreamConfig
    metrics     *MetricsAggregator
    wg          sync.WaitGroup
}
```

## Data Flow

### Test Initiation

```
1. Client: POST /api/v1/stream/start
   Body: { protocol: "tcp", direction: "download", duration: 30, streams: 4 }

2. API Gateway:
   - Validate request
   - Check rate limits
   - Generate stream_id (UUID)

3. Test Manager:
   - Create StreamState
   - Allocate resources
   - Start test goroutine

4. Test Engine:
   - Create ConnectionPool
   - Establish connections
   - Begin data transfer

5. Response:
   { stream_id: "uuid", websocket_url: "ws://...", test_server_tcp: "..." }
```

### Real-Time Metrics

```
1. Test Engine (per second):
   - Calculate metrics from all streams
   - Update StreamState.Metrics

2. Metrics Collector:
   - Aggregate stream metrics
   - Calculate percentiles

3. WebSocket Server:
   - Push metrics to connected clients
   - Format: { type: "metrics", data: {...} }
```

## Testing Modes

### Client Mode (CLI)

Client performs data transfer directly:

```
CLI                                Server
 │ POST /stream/start (mode:client) │
 ├─────────────────────────────────►│
 │◄── {test_server_tcp: "1.2.3.4"}  │
 │                                  │
 │ TCP connect to test server       │
 │══════════════════════════════════│
 │ Data transfer (measured locally) │
 │                                  │
 │ POST /stream/{id}/complete       │
 ├─────────────────────────────────►│
```

### Proxy Mode (Web)

Server performs test, streams metrics to browser:

```
Browser                            Server
 │ POST /stream/start (mode:proxy)  │
 ├─────────────────────────────────►│
 │◄── {websocket_url: "..."}        │
 │                                  │
 │ WebSocket connect                │
 │══════════════════════════════════│
 │◄══ Metrics stream                │
 │◄══ Complete                      │
```

## Performance

### Memory Management

**Buffer Pooling:**
```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 64*1024)
    },
}
```

**Optimizations:**
- Reuse buffers (avoid allocations in hot path)
- `io.CopyBuffer` with pre-allocated buffers
- Direct socket reads without copying

### Concurrency

- Goroutine per stream
- Lock-free metrics (atomic operations)
- WaitGroup for coordination
- Context for cancellation

### Network Stack

**Standard Sockets:**
- Go stdlib `net` package
- Good for up to ~10 Gbps

**Future: io_uring:**
- Linux async I/O
- Lower overhead than standard sockets

**Future: DPDK:**
- Kernel bypass for 25+ Gbps
- User-space packet processing

## Scalability

### Horizontal

- Load balancer distributes tests
- Each server handles subset of tests
- Registry service for discovery

### Vertical

- `MAX_CONCURRENT_TESTS` limit
- Per-test resource caps
- Timeout enforcement

## Security

### Rate Limiting

- Per-IP: configurable tests per minute
- Per-IP: concurrent test limit
- Global: server-wide concurrent limit

### Input Validation

- Duration: 1-300 seconds
- Streams: 1-64
- Packet size: 64-9000 bytes
- Protocol: enum validation

## Deployment

### Container

```dockerfile
FROM golang:1.25-alpine AS builder
COPY . .
RUN go build -o openbyte ./cmd/openbyte

FROM alpine:3.19
COPY --from=builder /app/openbyte /app/
EXPOSE 8080 8081 8082/tcp 8082/udp
CMD ["/app/openbyte", "server"]
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

## Monitoring

### Health Check

`GET /api/v1/health` returns server status.

### Metrics

- Active test count
- Server-wide throughput
- Error rate
- API response times

### Logging

Structured JSON logging with levels: DEBUG, INFO, WARN, ERROR.
