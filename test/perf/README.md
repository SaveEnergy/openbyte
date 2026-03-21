# Performance benchmarks

Package list: **`bench_packages.txt`**. Runner: **`scripts/perf/run_benchmarks.sh`**.

```bash
make perf-bench      # quick, stdout
make perf-record     # → build/perf/bench.txt (stable; default count=5)
make perf-compare    # needs test/perf/bench_baseline.txt + benchstat on PATH
make perf-check      # record + compare if baseline exists
```

Install comparison tool: `go install golang.org/x/perf/cmd/benchstat@latest`

Establish baseline once: `make perf-record && cp build/perf/bench.txt test/perf/bench_baseline.txt`

Nightly CI uploads **`build/perf/bench.txt`** as an artifact (see `AGENTS.md`).

## Autoresearch branch counter

**`autoresearch_counter.txt`** holds one integer: the **next** branch id **`N`**. New work branches are **`autoresearch/perf-N`**. After **`main`** has merged that branch, agents bump the file to **`N+1`**, commit on **`main`**, and **delete** **`autoresearch/perf-N`** locally and on **`origin`** (see **`AGENTS.md`**). If the file is missing, start at **`1`**.

## LLM experiment loop (optional)

See **`PROMPT_AUTORESEARCH.md`** for the full autoresearch-style prompt (branch, TSV logging, keep/discard rules).

In Cursor, use slash command **`/autoresearch`** (defined in **`.cursor/commands/autoresearch.md`**) to load the playbook; it points at `PROMPT_AUTORESEARCH.md` as the source of truth.
