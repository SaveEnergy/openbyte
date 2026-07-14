# openByte HTTP API

openByte is now HTTP-only. The old TCP/UDP stream API and websocket event feed were removed before 1.0.

Base URL: `http://localhost:8080`

## Agent quick flow

1. `GET /health` — confirm the server is alive.
2. `GET /api/v1/version` — read version and `server_name`.
3. `GET /api/v1/ping` — measure baseline latency and learn client IP.
4. `GET /api/v1/download?duration=5` — stream bytes for download measurement.
5. `POST /api/v1/upload` — send bytes for upload measurement.
6. `POST /api/v1/results` — optional: save/share a completed result.

The browser UI exposes the same reference at `/api.html`.

## Endpoints

### `GET /health`

Returns:

```json
{ "status": "ok" }
```

### `GET /api/v1/version`

Returns:

```json
{ "version": "v0.10.0", "server_name": "Frankfurt 25G" }
```

### `GET /api/v1/ping`

Returns latency probe metadata:

```json
{
  "pong": true,
  "timestamp": 1777890000000,
  "client_ip": "203.0.113.10",
  "ipv6": false
}
```

### `GET /api/v1/download?duration=5&chunk=1048576`

Streams `application/octet-stream` for up to `duration` seconds.

- `duration`: `1..MAX_TEST_DURATION` seconds, default `10` (server maximum defaults to `300s`)
- `chunk`: `65536..4194304`, default `1048576`

### `POST /api/v1/upload`

Consumes the request body as raw bytes until EOF or `MAX_TEST_DURATION` and returns:

```json
{ "bytes": 52428800, "duration_ms": 5020, "throughput_mbps": 83.55 }
```

### `POST /api/v1/results`

Saves one result object and returns an ID plus a same-origin relative share URL. The body is limited to 4096 bytes and unknown fields are rejected. If `Content-Type` is present, it must begin with `application/json`. Optional `diagnostics` metadata is accepted for client compatibility but is not persisted.

```json
{
  "download_mbps": 940.1,
  "upload_mbps": 912.4,
  "latency_ms": 8.2,
  "jitter_ms": 0.9,
  "loaded_latency_ms": 19.4,
  "bufferbloat_grade": "A",
  "ipv4": "203.0.113.10",
  "ipv6": "",
  "server_name": "Frankfurt 25G"
}
```

Successful response (`201 Created`):

```json
{ "id": "aB3dE7xQ", "url": "/results/aB3dE7xQ" }
```

### `GET /api/v1/results/{id}`

Fetches a saved result by its 8-character share ID. Stored results are purged after 90 days and may be removed earlier when `MAX_STORED_RESULTS` is exceeded.

## Errors

Errors produced by matched API handlers are JSON:

```json
{ "error": "message" }
```

The standard library may return plain-text `404` or `405` responses for an unmatched route or method. Version and result routes are rate-limited and return `429` with `Retry-After`; ping/download/upload are exempt from the request-rate limiter and instead use concurrency limits. openByte does not implement authentication, so put private deployments behind an authenticating reverse proxy.

See `api/openapi.yaml` for the machine-readable contract.
