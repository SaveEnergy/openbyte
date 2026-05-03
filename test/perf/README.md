# Performance benchmarks

Package list: **`bench_packages.txt`** (defaults include **`internal/{metrics,stream,websocket,api,jsonbody,results}`** and **`pkg/types`**). Runner: **`scripts/perf/run_benchmarks.sh`**.

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

## Autoresearch branch counter

**`autoresearch_counter.txt`** holds one integer: the **next** branch id **`N`**. New work branches are **`autoresearch/perf-N`**. After **`main`** has merged that branch, agents bump the file to **`N+1`**, commit on **`main`**, and **delete** **`autoresearch/perf-N`** locally and on **`origin`** (see **`AGENTS.md`**). If the file is missing, start at **`1`**.

## LLM experiment loop (optional)

See **`PROMPT_AUTORESEARCH.md`** for the full autoresearch-style prompt (branch, TSV logging, keep/discard rules).

**Cursor:** copy **`test/perf/AUTORESEARCH_CURSOR_COMMAND.md`** to **`.cursor/commands/autoresearch.md`** (or symlink), then use **`/autoresearch`**. That playbook points at **`PROMPT_AUTORESEARCH.md`** and starts with **`make autoresearch-preflight`**.
