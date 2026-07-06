# Performance benchmarks

Package list: **`bench_packages.txt`** (defaults include **`internal/{metrics,api,jsonbody,results}`** and **`pkg/types`**). Runner: **`scripts/perf/run_benchmarks.sh`**.

```bash
make perf-bench              # quick, stdout
make perf-record             # → build/perf/bench.txt (stable; default count=5)
make perf-compare            # needs baseline + current bench.txt; benchstat or go run fallback
make perf-check              # record + compare if baseline exists
make autoresearch-preflight  # exit 0 + AUTORESEARCH_* lines before a new perf-N branch
```

Optional (faster repeats): `go install golang.org/x/perf/cmd/benchstat@latest` — not required; **`make perf-compare`** uses **`go run golang.org/x/perf/cmd/benchstat@latest`** when `benchstat` is missing.

Establish baseline once: `make perf-record && cp build/perf/bench.txt test/perf/bench_baseline.txt`

Nightly CI uploads **`build/perf/bench.txt`** as an artifact (see `AGENTS.md`).

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

# CLI interleaved medians; add a saved baseline binary to BINS for A/B.
BINS="/tmp/openbyte-baseline ./bin/openbyte" RUNS=5 scripts/perf/run_cli_throughput.sh
```

Keep no-go experiments in the notes or PR body instead of shipping slower code.
For the 2026-07 browser/CLI pass: plain-HTTP request-stream uploads failed in
Chromium (`Failed to fetch`, use TLS/h2 for a real probe), multi-context
download sharding reduced aggregate throughput on the 4-vCPU Cloud VM, and CLI
single-request streaming upload was slower than discrete 4 MiB POSTs on Go's
HTTP/1.1 transport.

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

## Autoresearch branch counter

**`autoresearch_counter.txt`** holds one integer: the **next** branch id **`N`**. New work branches are **`autoresearch/perf-N`**. After **`main`** has merged that branch, agents bump the file to **`N+1`**, commit on **`main`**, and **delete** **`autoresearch/perf-N`** locally and on **`origin`** (see **`AGENTS.md`**). If the file is missing, start at **`1`**.

## LLM experiment loop (optional)

See **`PROMPT_AUTORESEARCH.md`** for the full autoresearch-style prompt (branch, TSV logging, keep/discard rules).

**Cursor:** copy **`test/perf/AUTORESEARCH_CURSOR_COMMAND.md`** to **`.cursor/commands/autoresearch.md`** (or symlink), then use **`/autoresearch`**. That playbook points at **`PROMPT_AUTORESEARCH.md`** and starts with **`make autoresearch-preflight`**.
