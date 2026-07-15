#!/usr/bin/env bash
# Run the curated microbenchmarks listed in test/perf/bench_packages.txt.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
packages_file="${PACKAGES_FILE:-$root/test/perf/bench_packages.txt}"
count="${BENCH_COUNT:-1}"
bench_time="${BENCH_TIME:-1s}"
packages=()

while IFS= read -r line || [[ -n "$line" ]]; do
  [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
  packages+=("$line")
done < "$packages_file"

cd "$root"
go test "${packages[@]}" -run '^$' -bench . -benchmem -benchtime="$bench_time" -count="$count"
