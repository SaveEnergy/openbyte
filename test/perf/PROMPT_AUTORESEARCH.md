# Autoresearch — performance benchmarks (LLM prompt)

Use this as a system or task prompt when an LLM should run **measured** perf experiments on openByte (microbenchmarks + `benchstat`), with **keep / discard / crash** logging to `results.tsv`.

**Cursor:** invoke **`/autoresearch`** to inject the short command that tells the agent to read this file and follow it (see **`.cursor/commands/autoresearch.md`**).

---

## Setup

To start a new run, align with the human on:

1. **Agree on a run tag** — propose from today’s date (e.g. `mar20`). Branch **`autoresearch/perf-<tag>`** must not exist yet (fresh run).
2. **Create the branch**: `git checkout -b autoresearch/perf-<tag>` from current `main` (or agreed base).
3. **Read in-scope context** (not the whole repo; expand only as needed):
   - **`AGENTS.md`** — Architecture § Performance, Build/CI perf notes, guardrails.
   - **`test/perf/README.md`** — how `perf-bench` / `perf-record` / `perf-compare` work.
   - **`scripts/perf/run_benchmarks.sh`** — flags, outputs (do **not** change without human OK).
   - **`test/perf/bench_packages.txt`** — which packages are in the suite (do **not** change without human OK).
   - **Hot-path code** you intend to touch (e.g. `internal/api`, `internal/websocket`, `internal/stream`, `internal/metrics`, `internal/jsonbody`) — read before editing.
4. **Verify toolchain**:
   - `go test` works for benchmark packages.
   - For comparisons: `benchstat` on PATH (`go install golang.org/x/perf/cmd/benchstat@latest`).
5. **Baseline**:
   - If **`test/perf/bench_baseline.txt`** is missing, the **first** successful `make perf-record` establishes it: copy `build/perf/bench.txt` → `test/perf/bench_baseline.txt` (after human confirms machine is “quiet” enough, or accept first run as provisional baseline).
6. **Initialize `results.tsv`**: create with header only (see below). First data row after the first completed experiment.
7. **Confirm and go**: human confirms branch + baseline policy; then start the loop.

---

## Experimentation

Each experiment is a **code change + measured benchmark run** on the same machine (minimize background load).

**Run the suite (stable numbers):**

```bash
make perf-record
```

Benchmark output is written to **`build/perf/bench.txt`** (the script redirects each `go test` there). Stdout is only the final `Wrote …` line; **stderr** still shows compile/test errors. To capture errors to a file:

```bash
make perf-record 2> build/perf/record.stderr
```

(Optional quick sanity: `make perf-bench` for a fast single pass — do **not** use it as the sole metric for keep/discard.)

**Compare to baseline (when `test/perf/bench_baseline.txt` exists):**

```bash
make perf-compare | tee build/perf/benchstat.log
```

(or redirect `> build/perf/benchstat.log 2>&1` if you do not need it on the console.)

**What you MAY do**

- Edit **application code** on agreed hot paths (handlers, websocket, stream, metrics, jsonbody, speedtest paths, etc.).
- Refactor **if** it reduces work per request / per tick / per allocation and **`go test ./... -short`** + **`make ci-lint`** stay green.

**What you MUST NOT do (without explicit human approval)**

- Change **`scripts/perf/run_benchmarks.sh`**, **`test/perf/bench_packages.txt`**, or **Makefile** perf targets (those are the “harness”; like `prepare.py` in the original).
- Add **new module dependencies** or change **`go.mod`** / tooling versions.
- Weaken **tests**, **timeouts**, or **correctness** to win benchmarks.
- Commit **`results.tsv`** (keep it **untracked** or gitignored locally — same spirit as original autoresearch).

**Goal**

Improve the benchmark suite on **ns/op**, **B/op**, and **allocs/op** where relevant — **lower is better** for all three. No single scalar like `val_bpb`; treat **regressions on any benchmark** in the suite as serious unless clearly explained (e.g. traded for a larger win elsewhere and human criteria say OK).

**Simplicity criterion** (same as original)

- Prefer **small, clear** changes. A tiny win that adds fragile complexity → probably **discard**.
- **Deleting** code with equal or better numbers → strong **keep**.
- Use **`benchstat`** output to judge noise vs signal; if uncertain, increase `BENCH_COUNT` once (e.g. `BENCH_COUNT=10 make perf-record`) before deciding.

**Quality gate (every experiment)**

Before logging **keep**:

```bash
make ci-lint
go test ./... -short
```

If either fails, fix or **discard** the experiment (revert commit).

---

## Output format (from `go test -bench`)

Benchmark lines look like:

```text
BenchmarkRespondJSON-8       1550601          765.1 ns/op       1153 B/op        14 allocs/op
```

Extract with e.g.:

```bash
grep '^Benchmark' build/perf/bench.txt
```

If **`make perf-record`** fails or **`build/perf/bench.txt`** has no `Benchmark` lines, treat as **crash** / failed run; inspect:

```bash
tail -n 80 build/perf/record.stderr   # if you captured stderr as above
tail -n 80 build/perf/bench.txt        # partial bench output or empty
```

---

## Logging results — `results.tsv`

Tab-separated (**not** comma-separated). **Do not commit** this file to git (repo **`.gitignore`** includes `test/perf/results.tsv`).

**Header row (tabs between fields):**

```text
commit	bench	ns_op	b_op	allocs	status	description
```

| Column | Meaning |
|--------|---------|
| `commit` | Short git hash (7 chars) |
| `bench` | Benchmark name (e.g. `BenchmarkRespondJSON`) — one row per benchmark you care about per experiment, or a single row `__SUITE__` if you only log `benchstat` summary (prefer **per-bench** rows for clarity) |
| `ns_op` | ns/op from the run (e.g. `765.1`) — use `0` if crash |
| `b_op` | B/op — use `0` if crash |
| `allocs` | allocs/op — use `0` if crash |
| `status` | `keep`, `discard`, or `crash` |
| `description` | Short note (what changed) |

**Example rows (tabs between fields):**

```text
commit	bench	ns_op	b_op	allocs	status	description
a1b2c3d	BenchmarkRespondJSON	765.1	1153	14	keep	baseline
b2c3d4e	BenchmarkRespondJSON	720.4	1100	12	keep	reuse buffer in respondJSON
c3d4e5f	BenchmarkRespondJSON	800.0	1200	15	discard	experimental unsafe string intern
d4e5f6g	BenchmarkRespondJSON	0	0	0	crash	nil deref in handler
```

---

## The experiment loop

Branch: **`autoresearch/perf-<tag>`** (e.g. `autoresearch/perf-mar20`).

**LOOP:**

1. Note current **commit** (experiment start).
2. Implement **one** idea in code (focused diff).
3. **`git commit`** with a message that matches the `description` you will log.
4. **`make ci-lint && go test ./... -short`** — if fail, fix or revert and log **crash**/**discard** as appropriate.
5. **`make perf-record`** (optionally **`2> build/perf/record.stderr`**).
6. **`grep '^Benchmark' build/perf/bench.txt`** — empty ⇒ **crash**; use **`tail`** on **`record.stderr`** / **`bench.txt`** to debug.
7. If baseline exists: **`make perf-compare > build/perf/benchstat.log 2>&1`** and read deltas.
8. Append row(s) to **`results.tsv`** (do not commit).
9. **Advance or revert:**
   - If the suite **improves** on your target benchmarks **without** unacceptable regressions elsewhere → **keep** (stay on this commit; optionally refresh `test/perf/bench_baseline.txt` from `build/perf/bench.txt` if human wants a new reference).
   - If **worse or neutral** with added complexity → **`git reset --hard`** to pre-experiment commit → **discard**.
   - **Crash** → revert or fix; log **crash** in TSV.

**Timeout**

- `make perf-record` with default `BENCH_COUNT=5` should finish in **well under 30 minutes** on a typical dev machine. If a run exceeds **45 minutes**, kill it, log **crash**, revert.

**Autonomy vs. safety**

- The original prompt says “never stop” — in practice, **token/session limits** apply. If interrupted, the human can resume from branch + `results.tsv`.
- Do **not** loop blindly if **every** idea regresses: widen diagnosis (read `benchstat.log`, pprof / `perf-smoke` only if human allows extra scope).

---

## Optional: anchor benchmarks

For simpler decisions, the human may name **1–3 anchor** benchmarks (e.g. `BenchmarkRespondJSON`, `BenchmarkEncodeMetricsMessage`). Optimize primarily for those; still **avoid** large regressions on the rest of the suite.

---

## See also

- **`test/perf/README.md`** — commands and baseline setup.
- **`AGENTS.md`** — performance architecture and nightly bench artifacts.
