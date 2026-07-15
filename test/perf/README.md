# Performance benchmarks

Package list: **`bench_packages.txt`** (currently **`internal/api`**, **`internal/results`**, and **`pkg/types`**). Runner: **`scripts/perf/run_benchmarks.sh`**.

```bash
make perf-bench              # quick, stdout
make perf-record             # → build/perf/bench.txt (stable; default count=5)
make perf-compare            # needs baseline + current bench.txt; benchstat or go run fallback
make perf-check              # record + compare if baseline exists
```

Optional (faster repeats): `go install golang.org/x/perf/cmd/benchstat@latest` — not required; **`make perf-compare`** uses **`go run golang.org/x/perf/cmd/benchstat@latest`** when `benchstat` is missing.

Establish baseline once: `make perf-record && cp build/perf/bench.txt test/perf/bench_baseline.txt`

Nightly CI runs the quick `make perf-bench` pass and keeps its output in the workflow log; it does not create `build/perf/bench.txt`.

## Throughput harnesses (manual)

These scripts measure end-to-end loopback throughput and are intentionally opt-in
because shared runners are noisy. Use interleaved medians when comparing changes.

```bash
make build
WEB_ROOT=./web ./bin/openbyte server

# Browser UI: real adaptive test, JSON lines + median summaries.
RUNS=5 scripts/perf/browser_throughput.mjs ui

# Browser probes for upload payload sizing, request streams, and sharding gates.
MODE=upload-blobs BLOB_MB=8,32,64 MAX_STREAMS=4 scripts/perf/browser_throughput.mjs
MODE=upload-stream MAX_STREAMS=4 scripts/perf/browser_throughput.mjs
MODE=download-shards SHARDS=1,2,4 scripts/perf/browser_throughput.mjs
MODE=upload-shards SHARDS=1,2,4 scripts/perf/browser_throughput.mjs

# Direct generated TLS, with h2 enabled and disabled.
PORT=8444 BIND_ADDRESS=127.0.0.1 TLS_AUTO_GEN=1 WEB_ROOT=./web ./bin/openbyte server
URL=https://localhost:8444/ IGNORE_HTTPS_ERRORS=1 RUNS=3 scripts/perf/browser_throughput.mjs ui
PORT=8445 BIND_ADDRESS=127.0.0.1 TLS_AUTO_GEN=1 HTTP2_ENABLED=false WEB_ROOT=./web ./bin/openbyte server
URL=https://localhost:8445/ IGNORE_HTTPS_ERRORS=1 RUNS=3 scripts/perf/browser_throughput.mjs ui

# Local self-signed TLS/h2 proxy for browser request-stream experiments.
go run scripts/perf/h2_reverse_proxy.go -target http://localhost:8080 -addr 127.0.0.1:8443
URL=https://localhost:8443/ IGNORE_HTTPS_ERRORS=1 MODE=upload-stream MAX_STREAMS=4 scripts/perf/browser_throughput.mjs

# Traefik ALPN comparison. The bundled compose files default to openbyte-h1@file;
# set TRAEFIK_TLS_OPTIONS=openbyte-h2@file for the h2 comparison run.
URL=https://localhost/ IGNORE_HTTPS_ERRORS=1 RUNS=3 scripts/perf/browser_throughput.mjs ui

```

Keep no-go experiments in the notes or PR body instead of shipping slower code.
For the 2026-07 browser pass: plain-HTTP request-stream uploads failed in
Chromium (`Failed to fetch`, use TLS/h2 for a real probe), multi-context
download sharding reduced aggregate throughput on the 4-vCPU Cloud VM.

The local TLS/h2 proxy makes request-stream uploads work, but they were not a
clear production win on the Cloud VM: Blob uploads and streaming uploads both
landed around **5.5-5.9 Gbit/s** at 1-8 streams, with streaming only ~1-3%
faster at 1-4 streams and slower at 8 streams. The full UI through the local
h2 proxy measured **8.54 Gbit/s download / 5.76 Gbit/s upload** median, below
plain HTTP loopback, so do not ship browser streaming uploads without target
hardware measurements showing a real gain.

Direct generated TLS separated proxy cost from protocol cost on the Cloud VM:
HTTP/2 measured **11.26 Gbit/s download / 8.77 Gbit/s upload** median, while
HTTP/1-only (`HTTP2_ENABLED=false`) measured **20.38 Gbit/s download /
13.11 Gbit/s upload** median. Browser HTTP/1 ramps are capped at six streams to
avoid measuring queued phantom streams beyond Chromium's per-origin connection
limit. Upload sharding was only a modest gate result (**11.56 -> 13.56 Gbit/s**
plain HTTP, **9.87 -> 11.20 Gbit/s** direct TLS h1), so no production sharding
refactor is justified on this VM.

The bundled Traefik compose overlays use `openbyte-h1@file` by default for the
openByte HTTPS routers. That keeps browser speed tests off the slower h2 path
while preserving `openbyte-h2@file` as an explicit comparison mode. Local
Traefik on the Cloud VM measured **15.11 Gbit/s download / 12.66 Gbit/s upload**
median with h1-only ALPN, versus **9.95 Gbit/s / 7.97 Gbit/s** with h2 ALPN.
