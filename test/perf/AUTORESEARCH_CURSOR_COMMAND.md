---
description: Measured perf autoresearch — benchmarks, benchstat, results.tsv; --loop = multi-iteration (no hop-out after loop-complete)
---

You are running **openByte perf autoresearch**: microbenchmark experiments, `benchstat` comparisons, and structured logging to `test/perf/results.tsv`.

**Loop mode:** if the human invoked **`/autoresearch --loop`** (or **`--loop`** appears in the slash-command args), treat **outer + inner** iterations as one task. **Invalid:** run **`make autoresearch-loop-complete`** once and **stop** — that is a **hop-out** and does **not** honor **`--loop`**.

**Valid cycle (repeat until blocked):** (1) On merge-ready **`autoresearch/perf-N`**, clean tree, counter **`N`**: **`make autoresearch-loop-complete`**. (2) **Push** **`main`**; delete remote **`autoresearch/perf-N`** if present. (3) **`make autoresearch-preflight`**. (4) On **`autoresearch/perf-(N+1)`**, **Setup** §3–6: baseline **`make perf-record`**, baseline / **`results.tsv`**, then **experiment loop** ( **`make perf-record`**, **`make perf-compare`**, commits, logging). (5) When the new branch is merge-ready, return to (1).

**Stop** on merge conflict, failed script/tests, ambiguous correctness, or human stop. **Do not** blindly re-merge if every experiment regresses — widen diagnosis first (same as non-loop prompt).

## Bootstrap (run first; exit code is the contract)

```bash
make autoresearch-preflight
```

- **Exit 0:** stdout lines `AUTORESEARCH_NEXT_N=…`, `AUTORESEARCH_BRANCH=…`, `AUTORESEARCH_BENCHSTAT_CMD=…` — use them. `make perf-compare` already falls back if `benchstat` is missing; the printed command matches that fallback.
- **Non-zero:** fix the reported conflict (duplicate local branch while on another branch, bad counter, missing files) before experimenting.
- **Resume:** if your **current** branch is already **`autoresearch/perf-N`** matching the counter, preflight still **exits 0** (remote may still list that branch).

## Source of truth

Read **`test/perf/PROMPT_AUTORESEARCH.md`** and follow it for setup, the experiment loop, **`--loop`** (merge + next iteration), `results.tsv` schema, git cleanup after merge, and “See also” references.

## Non-negotiables (summary)

- One experiment → one commit; log every attempt to **`test/perf/results.tsv`** (gitignored).
- **`make perf-record`** for recorded runs; extract benchmark lines from **`build/perf/bench.txt`** (not stdout alone).
- Do **not** change **`scripts/perf/run_benchmarks.sh`** or **`test/perf/bench_packages.txt`** without explicit human approval.
- **Default:** pick the next optimization target from evidence and suite deltas; do **not** ask the human to choose a benchmark unless they named one or you are blocked.

## Install this file as a Cursor slash command

Repo **`.cursor/`** is gitignored. Copy or symlink this file to **`.cursor/commands/autoresearch.md`** in your clone so **`/autoresearch`** loads this playbook.
