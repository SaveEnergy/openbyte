# openByte Architecture

openByte is a browser-first HTTP speed test server. The product deliberately stays lean: one `openbyte` binary, embedded web UI, HTTP APIs, and a small SQLite-backed results store.

## Runtime shape

```text
Browser / check / agent
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
- Speed tests use HTTP `/api/v1/download`, `/api/v1/upload`, and `/api/v1/ping` only.
- Browser tests run in a module Web Worker where supported.
- Adaptive ramping saturates the link, then measures using the selected stream count.
- Any new top-level web asset (HTML, JavaScript, CSS, SVG, and so on) must be added to the static allowlist; font files under `web/fonts/` are allowlisted by extension.

## Backend

- Routing uses stdlib `net/http.ServeMux` method patterns.
- Download/upload handlers enforce bounded concurrency, per-IP limits, configured maximum duration, body deadlines, and body draining on error paths; download chunk requests are also range-checked. Upload bodies are read until EOF or the configured deadline and do not have a byte limit.
- Results use pure-Go SQLite (`modernc.org/sqlite`) with WAL mode, 90-day retention, max-count cleanup, and cancellation-aware lock retries.
- Config comes from defaults, environment, then CLI flags.

## Deployment

- Docker exposes only port `8080`.
- Traefik deployments route HTTP(S) to internal port `8080`, keep dedicated upload routers unbuffered, and default openByte HTTPS ALPN to HTTP/1.1.
- `SERVER_NAME` controls the server display name returned by `/api/v1/version` and shown in the UI.
