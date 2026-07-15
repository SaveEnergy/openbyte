# openByte Architecture

openByte is a browser-first HTTP speed test server. The product deliberately stays lean: one `openbyte` binary, embedded web UI, HTTP APIs, and a small SQLite-backed results store.

## Runtime shape

```text
Browser / HTTP client
         │
         ▼
openbyte (:8080)
  ├─ Web UI + static assets
  ├─ HTTP speed APIs: ping, download, upload
  ├─ Shared-results API
  └─ SQLite results store
```

There are no separate TCP/UDP test ports and no websocket stream API. Those pre-1.0 paths were cut in favor of the adaptive HTTP/browser strategy.

## Public API

[`api/openapi.yaml`](api/openapi.yaml) is the canonical API contract; the server
does not duplicate it as a browser page. There is no `/api/v1/version` route:
the UI obtains the configured server name from its same-origin
`/api/v1/ping?meta=1` bootstrap request.

## Frontend

- Static assets are embedded with `//go:embed` and can be overridden with `WEB_ROOT` for development.
- Speed tests use HTTP `/api/v1/download`, `/api/v1/upload`, and `/api/v1/ping` only.
- Browser tests run in a module Web Worker where supported.
- Adaptive ramping saturates the link, then measures using the selected stream count.
- Client IP discovery stays eager on page load. `/api/v1/ping` alone allows cross-origin reads so the UI can probe dedicated IPv4/IPv6 hostnames; all other API routes are same-origin.
- Static serving derives its allowed paths from `web/embed.go`; `WEB_ROOT` can override those files but cannot expose additional paths.

## Backend

- Routing uses stdlib `net/http.ServeMux` method patterns.
- Download/upload handlers enforce bounded concurrency, per-IP limits, configured maximum duration, body deadlines, and body draining on error paths; download chunk requests are also range-checked. Upload bodies are read until EOF or the configured deadline and do not have a byte limit.
- Results use pure-Go SQLite (`modernc.org/sqlite`) with WAL mode, 90-day retention, max-count cleanup, and cancellation-aware lock retries.
- Config comes from defaults and environment variables.

## Deployment

- Docker exposes only port `8080`.
- Traefik deployments route HTTP(S) through one generic router per entrypoint to internal port `8080` and default openByte HTTPS ALPN to HTTP/1.1.
- `SERVER_NAME` controls the server display name returned by bootstrap ping metadata and shown in the UI.
