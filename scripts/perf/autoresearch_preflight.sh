#!/usr/bin/env bash
# Verifiable autoresearch bootstrap: exit codes + AUTORESEARCH_* lines on stdout.
# Usage: ./scripts/perf/autoresearch_preflight.sh   OR   make autoresearch-preflight
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

die() { echo "autoresearch_preflight: $*" >&2; exit 1; }

command -v go >/dev/null 2>&1 || die "go not on PATH"
test -f test/perf/bench_packages.txt || die "missing test/perf/bench_packages.txt"
test -f scripts/perf/run_benchmarks.sh || die "missing scripts/perf/run_benchmarks.sh"

COUNTER="test/perf/autoresearch_counter.txt"
N=1
if [[ -f "$COUNTER" ]]; then
	N="$(sed '/^[[:space:]]*$/d' "$COUNTER" | head -n1 | tr -d '[:space:]')"
fi
[[ "$N" =~ ^[1-9][0-9]*$ ]] || die "invalid next id in $COUNTER (expected positive integer): '$N'"

BRANCH="autoresearch/perf-$N"
CURRENT="$(git branch --show-current 2>/dev/null || true)"
if git show-ref --verify --quiet "refs/heads/$BRANCH"; then
	if [[ "$CURRENT" != "$BRANCH" ]]; then
		die "local branch $BRANCH already exists — checkout it to resume, or delete it before starting fresh"
	fi
fi

if git remote get-url origin >/dev/null 2>&1; then
	if out="$(git ls-remote --heads origin "$BRANCH" 2>/dev/null)" && [[ -n "$out" ]]; then
		if [[ "$CURRENT" != "$BRANCH" ]]; then
			die "origin already has $BRANCH — delete remote branch, merge, or bump counter after merge"
		fi
	fi
fi

PKG_LINES="$(grep -Ev '^[[:space:]]*#|^[[:space:]]*$' test/perf/bench_packages.txt | wc -l | awk '{print $1}')"

if command -v benchstat >/dev/null 2>&1; then
	BENCHSTAT_CMD="benchstat"
else
	BENCHSTAT_CMD="go run golang.org/x/perf/cmd/benchstat@latest"
fi

echo "AUTORESEARCH_NEXT_N=$N"
echo "AUTORESEARCH_BRANCH=$BRANCH"
echo "AUTORESEARCH_BENCHSTAT_CMD=$BENCHSTAT_CMD"
echo "AUTORESEARCH_BENCH_PACKAGES=$PKG_LINES"
echo "autoresearch_preflight: OK (create $BRANCH from main when ready)"
