#!/usr/bin/env bash
# Unified microbenchmark runner for reproducible output (benchstat-friendly).
# Usage:
#   scripts/perf/run_benchmarks.sh              # -> build/perf/bench.txt
#   scripts/perf/run_benchmarks.sh --stdout     # print only (fast local loop)
#
# Env:
#   BENCH_OUT   output file (default: build/perf/bench.txt); ignored with --stdout
#   BENCH_COUNT go test -count (default: 5 for stable medians)
#   BENCH_TIME  go test -benchtime (default: 1s)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

PACKAGES_FILE="${PACKAGES_FILE:-test/perf/bench_packages.txt}"
COUNT="${BENCH_COUNT:-5}"
TIME="${BENCH_TIME:-1s}"
OUT="${BENCH_OUT:-build/perf/bench.txt}"

if [[ ! -f "$PACKAGES_FILE" ]]; then
	echo "error: missing $PACKAGES_FILE" >&2
	exit 1
fi

run_pkg() {
	local pkg="$1"
	# -run '^$' runs no tests; only benchmarks.
	go test "$pkg" -run '^$' -bench . -benchmem -benchtime="$TIME" -count="$COUNT"
	return 0
}

if [[ "${1:-}" == "--stdout" ]]; then
	while IFS= read -r line || [[ -n "$line" ]]; do
		[[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
		run_pkg "$line"
	done <"$PACKAGES_FILE"
	exit 0
fi

mkdir -p "$(dirname "$OUT")"
: >"$OUT"
while IFS= read -r line || [[ -n "$line" ]]; do
	[[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
	run_pkg "$line" >>"$OUT"
done <"$PACKAGES_FILE"

echo "Wrote $OUT (count=$COUNT benchtime=$TIME)"
