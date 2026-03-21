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

## LLM experiment loop (optional)

See **`PROMPT_AUTORESEARCH.md`** for the full autoresearch-style prompt (branch, TSV logging, keep/discard rules).

In Cursor, use slash command **`/autoresearch`** (defined in **`.cursor/commands/autoresearch.md`**) to load the playbook; it points at `PROMPT_AUTORESEARCH.md` as the source of truth.
