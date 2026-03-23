#!/usr/bin/env bash
# Finish one autoresearch iteration: merge autoresearch/perf-N into main, bump counter,
# delete the local feature branch, create autoresearch/perf-(N+1) from updated main.
#
# Usage (repo root): on branch autoresearch/perf-N, clean working tree.
# Optional: AUTORESEARCH_BASE_BRANCH=main (default: main)
#
# Does not push. After success, push main and delete remote autoresearch/perf-N if you use a remote.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

BASE="${AUTORESEARCH_BASE_BRANCH:-main}"
COUNTER="test/perf/autoresearch_counter.txt"

CURRENT="$(git branch --show-current 2>/dev/null || true)"
if [[ ! "$CURRENT" =~ ^autoresearch/perf-([1-9][0-9]*)$ ]]; then
	echo "autoresearch_loop_complete: must be on autoresearch/perf-N (got: ${CURRENT:-detached})" >&2
	exit 1
fi
N="${BASH_REMATCH[1]}"

if ! git diff --quiet || ! git diff --cached --quiet; then
	echo "autoresearch_loop_complete: working tree not clean — commit or stash first" >&2
	exit 1
fi

if [[ ! -f "$COUNTER" ]]; then
	echo "autoresearch_loop_complete: missing $COUNTER" >&2
	exit 1
fi

COUNTER_N="$(sed '/^[[:space:]]*$/d' "$COUNTER" | head -n1 | tr -d '[:space:]')"
if [[ "$COUNTER_N" != "$N" ]]; then
	echo "autoresearch_loop_complete: branch perf-$N but $COUNTER contains next id $COUNTER_N (fix counter or branch)" >&2
	exit 1
fi

if ! git show-ref --verify --quiet "refs/heads/$BASE"; then
	echo "autoresearch_loop_complete: base branch '$BASE' does not exist locally" >&2
	exit 1
fi

git checkout "$BASE"

if git remote get-url origin >/dev/null 2>&1; then
	git fetch origin
	if git show-ref --verify --quiet "refs/remotes/origin/$BASE"; then
		git merge --ff-only "origin/$BASE" || {
			echo "autoresearch_loop_complete: $BASE is not fast-forward to origin/$BASE — update $BASE and retry" >&2
			exit 1
		}
	fi
fi

git merge --no-ff "autoresearch/perf-$N" -m "Merge branch 'autoresearch/perf-$N'"

if [[ "${AUTORESEARCH_LOOP_SKIP_TESTS:-}" != "1" ]]; then
	make ci-lint
	go test ./... -short
fi

NEXT=$((N + 1))
printf '%s\n' "$NEXT" >"$COUNTER"

git add "$COUNTER"
git commit -m "chore(perf): bump autoresearch counter after perf-$N merge"

git branch -d "autoresearch/perf-$N"

git checkout -b "autoresearch/perf-$NEXT"

echo "autoresearch_loop_complete: OK — on autoresearch/perf-$NEXT (merged perf-$N into $BASE, counter=$NEXT)"
echo "autoresearch_loop_complete: push when ready: git push origin $BASE && git push origin --delete autoresearch/perf-$N (if remote branch exists)"
