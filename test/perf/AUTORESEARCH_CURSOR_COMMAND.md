---
description: Measured perf autoresearch — benchmarks, benchstat, results.tsv (openByte)
---

You are running **openByte perf autoresearch**: microbenchmark experiments, `benchstat` comparisons, and structured logging to `test/perf/results.tsv`.

## Bootstrap (run first; exit code is the contract)

```bash
make autoresearch-preflight
```

- **Exit 0:** stdout lines `AUTORESEARCH_NEXT_N=…`, `AUTORESEARCH_BRANCH=…`, `AUTORESEARCH_BENCHSTAT_CMD=…` — use them. `make perf-compare` already falls back if `benchstat` is missing; the printed command matches that fallback.
- **Non-zero:** fix the reported conflict (duplicate local branch while on another branch, bad counter, missing files) before experimenting.
- **Resume:** if your **current** branch is already **`autoresearch/perf-N`** matching the counter, preflight still **exits 0** (remote may still list that branch).

## Source of truth

Read **`test/perf/PROMPT_AUTORESEARCH.md`** and follow it for setup, the experiment loop, `results.tsv` schema, **manual** merge/counter steps after a perf branch lands on **`main`**, and “See also” references.

## Non-negotiables (summary)

- One experiment → one commit; log every attempt to **`test/perf/results.tsv`** (gitignored).
- **`make perf-record`** for recorded runs; extract benchmark lines from **`build/perf/bench.txt`** (not stdout alone).
- Do **not** change **`scripts/perf/run_benchmarks.sh`** or **`test/perf/bench_packages.txt`** without explicit human approval.
- **Default:** pick the next optimization target from evidence and suite deltas; do **not** ask the human to choose a benchmark unless they named one or you are blocked.

## Install this file as a Cursor slash command

Repo **`.cursor/`** is gitignored. Copy or symlink this file to **`.cursor/commands/autoresearch.md`** in your clone so **`/autoresearch`** loads this playbook.
