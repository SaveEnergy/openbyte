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

- `duration`: `1..300`, default `10`
- `chunk`: `65536..4194304`, default `1048576`

### `POST /api/v1/upload`

Consumes `application/octet-stream` and returns:

```json
{ "bytes": 52428800, "duration_ms": 5020, "throughput_mbps": 83.55 }
```

### `POST /api/v1/results`

Saves one result object and returns an ID + share URL.

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

### `GET /api/v1/results/{id}`

Fetches a saved result by its 8-character share ID.

## Errors

Errors are JSON:

```json
{ "error": "message" }
```

See `api/openapi.yaml` for the machine-readable contract.
