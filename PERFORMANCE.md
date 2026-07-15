# Performance Guide

Measure the HTTP data path before changing it. The browser worker, reverse proxy,
and protocol choice have much larger effects than nanosecond middleware changes.

## Curated benchmarks

```bash
make perf-bench
```

The suite covers transfer read/write loops, cached gzip assets, JSON handling,
ping responses, and SQLite result save/get. It intentionally excludes trivial
predicates and has no pretend regression gate without a maintained baseline.

## Profiling

```bash
make build
PPROF_ENABLED=true PPROF_ADDR=127.0.0.1:6060 ./bin/openbyte
curl -sf http://127.0.0.1:8080/api/v1/ping
curl -s "http://127.0.0.1:6060/debug/pprof/profile?seconds=10" -o /tmp/openbyte-cpu.pprof
go tool pprof /tmp/openbyte-cpu.pprof
```

Use the browser UI for end-to-end upload/download profiling. Manual browser,
TLS/h2, proxy, and sharding harnesses are documented in
[`test/perf/README.md`](test/perf/README.md).

## Guardrails

- Compare interleaved medians on the target hardware.
- Preserve cancellation, concurrency limits, random payloads, warm-up gating,
  adaptive streams, and result sharing unless evidence says otherwise.
- Keep advanced telemetry opt-in and server-side.
- Do not ship benchmark-only complexity for a marginal or noisy result.
