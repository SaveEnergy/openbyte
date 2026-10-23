# Performance Guide

Measure the HTTP data path before changing it. The browser worker, reverse proxy,
and protocol choice have much larger effects than nanosecond middleware changes.

## Curated benchmarks

```bash
make perf-bench
```

The suite covers transfer read/write loops, cached gzip assets, JSON handling,
ping responses, complete upload-handler lifecycle, and SQLite result save/get.
It intentionally excludes trivial predicates and has no pretend regression gate
without a maintained baseline.

`BenchmarkUploadHandler` covers 64 KiB and 8 MiB bodies with concurrency policy,
deadline setup, and teardown. Its in-memory byte rate is not network throughput.
Compare allocation counts separately from elapsed time, and use `-cpu=1,4` to
check both serial and concurrent request overhead.

## Profiling

```bash
make build
DATA_DIR=./.agent/perf-data PPROF_ENABLED=true PPROF_ADDR=127.0.0.1:6060 ./bin/openbyte
curl -sf http://127.0.0.1:8080/api/v1/ping
curl -s "http://127.0.0.1:6060/debug/pprof/profile?seconds=10" -o /tmp/openbyte-cpu.pprof
go tool pprof /tmp/openbyte-cpu.pprof
```

Use the browser UI for end-to-end upload/download profiling. Manual browser,
TLS/h2, proxy, and sharding harnesses are documented in
[`test/perf/README.md`](test/perf/README.md).
Keep profiling data separate from development or production databases: opening
the results store runs retention cleanup. The example uses an ignored directory.

## Guardrails

- Compare interleaved medians on the target hardware.
- Profile plain HTTP, direct TLS, and the deployed proxy separately. A browser
  or same-host client can limit throughput before the Go server saturates;
  confirm server capacity with a separate load-generating host.
- Preserve cancellation, concurrency limits, random payloads, warm-up gating,
  adaptive streams, and result sharing unless evidence says otherwise.
- Keep advanced telemetry opt-in and server-side.
- Do not ship benchmark-only complexity for a marginal or noisy result.
