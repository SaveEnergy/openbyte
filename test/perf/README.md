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

# CLI interleaved medians; add a saved baseline binary to BINS for A/B.
BINS="/tmp/openbyte-baseline ./bin/openbyte" RUNS=5 scripts/perf/run_cli_throughput.sh
```

Keep no-go experiments in the notes or PR body instead of shipping slower code.
For the 2026-07 browser/CLI pass: plain-HTTP request-stream uploads failed in
Chromium (`Failed to fetch`, use TLS/h2 for a real probe), multi-context
download sharding reduced aggregate throughput on the 4-vCPU Cloud VM, and CLI
single-request streaming upload was slower than discrete 4 MiB POSTs on Go's
HTTP/1.1 transport.

## Autoresearch branch counter

**`autoresearch_counter.txt`** holds one integer: the **next** branch id **`N`**. New work branches are **`autoresearch/perf-N`**. After **`main`** has merged that branch, agents bump the file to **`N+1`**, commit on **`main`**, and **delete** **`autoresearch/perf-N`** locally and on **`origin`** (see **`AGENTS.md`**). If the file is missing, start at **`1`**.

## LLM experiment loop (optional)

See **`PROMPT_AUTORESEARCH.md`** for the full autoresearch-style prompt (branch, TSV logging, keep/discard rules).

**Cursor:** copy **`test/perf/AUTORESEARCH_CURSOR_COMMAND.md`** to **`.cursor/commands/autoresearch.md`** (or symlink), then use **`/autoresearch`**. That playbook points at **`PROMPT_AUTORESEARCH.md`** and starts with **`make autoresearch-preflight`**.
