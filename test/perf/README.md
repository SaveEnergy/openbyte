# Performance harnesses

Run the curated microbenchmarks:

```bash
make perf-bench
BENCH_COUNT=5 BENCH_TIME=2s make perf-bench
```

The package list is [`bench_packages.txt`](bench_packages.txt). Compare saved
outputs with `benchstat` manually when an experiment has an explicit baseline.

## End-to-end browser measurements

```bash
make build
WEB_ROOT=./web ./bin/openbyte server

# Real adaptive UI test with JSON output and median summaries.
RUNS=5 scripts/perf/browser_throughput.mjs ui

# Opt-in experiments.
MODE=upload-blobs BLOB_MB=8,32,64 MAX_STREAMS=4 scripts/perf/browser_throughput.mjs
MODE=upload-stream MAX_STREAMS=4 scripts/perf/browser_throughput.mjs
MODE=download-shards SHARDS=1,2,4 scripts/perf/browser_throughput.mjs
MODE=upload-shards SHARDS=1,2,4 scripts/perf/browser_throughput.mjs
```

For direct TLS protocol comparisons:

```bash
PORT=8444 BIND_ADDRESS=127.0.0.1 TLS_AUTO_GEN=1 WEB_ROOT=./web ./bin/openbyte server
URL=https://localhost:8444/ IGNORE_HTTPS_ERRORS=1 RUNS=5 scripts/perf/browser_throughput.mjs ui

PORT=8445 BIND_ADDRESS=127.0.0.1 TLS_AUTO_GEN=1 HTTP2_ENABLED=false WEB_ROOT=./web ./bin/openbyte server
URL=https://localhost:8445/ IGNORE_HTTPS_ERRORS=1 RUNS=5 scripts/perf/browser_throughput.mjs ui
```

The local h2 proxy remains available for request-stream experiments:

```bash
go run scripts/perf/h2_reverse_proxy.go -target http://localhost:8080 -addr 127.0.0.1:8443
URL=https://localhost:8443/ IGNORE_HTTPS_ERRORS=1 MODE=upload-stream scripts/perf/browser_throughput.mjs
```

Previous Cloud VM measurements favored HTTP/1.1 and did not show a production
sharding or request-streaming win. Treat that as a hypothesis, not a universal
result: use interleaved medians on the intended 25G hardware before changing the
production transfer strategy.
