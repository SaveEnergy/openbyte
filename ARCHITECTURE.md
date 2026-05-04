# openByte Architecture

openByte is a browser-first HTTP speed test server. The product deliberately stays lean: one `openbyte` binary, embedded web UI, HTTP APIs, and a small SQLite-backed results store.

## Runtime shape

```text
Browser / CLI / agent
        │
        ▼
openbyte server (:8080)
  ├─ Web UI + static assets
  ├─ HTTP speed APIs: ping, download, upload
  ├─ Version/API docs/results APIs
  └─ SQLite results store
```

There are no separate TCP/UDP test ports and no websocket stream API. Those pre-1.0 paths were cut in favor of the adaptive HTTP/browser strategy.

## Public API

```text
GET  /health
GET  /api/v1/version
GET  /api/v1/ping
GET  /api/v1/download
POST /api/v1/upload
POST /api/v1/results
GET  /api/v1/results/{id}
```

The human/agent quick reference is served at `/api.html`; the machine-readable contract lives in `api/openapi.yaml`.

## Frontend

- Static assets are embedded with `//go:embed` and can be overridden with `WEB_ROOT` for development.
- Speed tests use HTTP `/download`, `/upload`, and `/ping` only.
- Browser tests run in a module Web Worker where supported.
- Adaptive ramping saturates the link, then measures using the selected stream count.
- New top-level `web/*.js` / `web/*.css` files must be added to the static allowlist.

## Backend

- Routing uses stdlib `net/http.ServeMux` method patterns.
- Download/upload handlers enforce bounded concurrency, per-IP limits, size/duration limits, and body draining on error paths.
- Results use pure-Go SQLite (`modernc.org/sqlite`) with WAL mode, retention, and max-count cleanup.
- Config comes from defaults, environment, then CLI flags.

## Deployment

- Docker exposes only port `8080`.
- Traefik deployments route HTTP(S) to internal port `8080` and keep the upload body-limit middleware.
- `SERVER_NAME` controls the server display name returned by `/api/v1/version` and shown in the UI.
