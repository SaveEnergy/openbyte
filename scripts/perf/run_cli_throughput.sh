#!/usr/bin/env bash
# Interleaved CLI throughput runner for noisy loopback measurements.
#
# Examples:
#   make build
#   BINS="./bin/openbyte" scripts/perf/run_cli_throughput.sh
#   BINS="/tmp/openbyte-baseline ./bin/openbyte" RUNS=7 scripts/perf/run_cli_throughput.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

SERVER_URL="${SERVER_URL:-http://localhost:8080}"
BINS="${BINS:-./bin/openbyte}"
RUNS="${RUNS:-5}"
DURATION="${DURATION:-3}"
STREAMS="${STREAMS:-4}"
DIRECTIONS="${DIRECTIONS:-download upload}"

extract_mbps() {
	awk -F= '/^throughput_avg_mbps=/{print $2; found=1} END{if (!found) exit 1}'
}

median() {
	python3 - "$@" <<'PY'
import statistics
import sys

values = [float(arg) for arg in sys.argv[1:] if arg]
if not values:
    print("0")
else:
    print(f"{statistics.median(values):.1f}")
PY
}

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

for direction in $DIRECTIONS; do
	echo "direction=$direction server=$SERVER_URL duration=$DURATION streams=$STREAMS"
	for run in $(seq 1 "$RUNS"); do
		for bin in $BINS; do
			value="$("$bin" client \
				-d "$direction" \
				-t "$DURATION" \
				-s "$STREAMS" \
				--plain \
				--no-progress \
				"$SERVER_URL" | extract_mbps)"
			name="$(basename "$bin")"
			printf 'run=%s bin=%s mbps=%s\n' "$run" "$name" "$value"
			printf '%s %s %s\n' "$direction" "$name" "$value" >>"$tmp"
		done
	done
	for bin in $BINS; do
		name="$(basename "$bin")"
		values="$(awk -v d="$direction" -v b="$name" '$1 == d && $2 == b { print $3 }' "$tmp")"
		# shellcheck disable=SC2086
		printf 'summary direction=%s bin=%s median_mbps=%s\n' "$direction" "$name" "$(median $values)"
	done
done
