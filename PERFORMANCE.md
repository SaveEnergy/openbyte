# Performance Guide

openByte is now HTTP-only. Performance work should focus on the browser/HTTP data path, JSON/result APIs, and runtime behavior around `/api/v1/download`, `/api/v1/upload`, and `/api/v1/ping`.

## Benchmarks

```bash
make perf-bench   # quick stdout pass
make perf-record  # writes build/perf/bench.txt
make perf-compare # benchstat baseline vs current
```

The benchmark package list lives in `test/perf/bench_packages.txt`.

## Local smoke profiling

```bash
make build
PPROF_ENABLED=true PPROF_ADDR=127.0.0.1:6060 ./bin/openbyte server
curl -sf http://127.0.0.1:8080/api/v1/ping
curl -s "http://127.0.0.1:6060/debug/pprof/profile?seconds=10" -o /tmp/openbyte-cpu.pprof
go tool pprof /tmp/openbyte-cpu.pprof
```

For upload/download smoke, use the Web UI or simple HTTP clients against:

```bash
curl -o /dev/null "http://127.0.0.1:8080/api/v1/download?duration=5"
head -c 33554432 /dev/zero | curl -X POST --data-binary @- \
  -H 'Content-Type: application/octet-stream' \
  http://127.0.0.1:8080/api/v1/upload
```

## Hot paths

- `internal/api/speedtest_download.go`
- `internal/api/speedtest_upload.go`
- `internal/api/speedtest_handlers.go`
- `internal/jsonbody/decode.go`
- `internal/results/*`
- `web/speedtest-worker.js`
- `web/speedtest-adaptive.js`
- `web/speedtest-http-*.js`

## Rules of thumb

- Correctness first; profile before optimizing.
- Do not add user-visible telemetry by default; keep detail opt-in.
- Avoid benchmark-only complexity unless the win is large and easy to explain.
- Keep the default product path simple: browser HTTP speed test + result sharing.
