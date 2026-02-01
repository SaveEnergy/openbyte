## Performance Profiling Guide

### Enable pprof

Set env vars and run server:

```
PPROF_ENABLED=true PPROF_ADDR=127.0.0.1:6060 go run ./cmd/openbyte server
```

Endpoints:

- `http://127.0.0.1:6060/debug/pprof/`
- `http://127.0.0.1:6060/debug/pprof/profile?seconds=30`

### Runtime stats logging

```
PERF_STATS_INTERVAL=5s go run ./cmd/openbyte server
```

Logs include goroutines, heap usage, GC count, and pause totals.

### Local load generator

Build:

```
go build -o bin/openbyte-load ./cmd/loadtest
```

Examples:

```
./bin/openbyte-load --mode tcp-download --host 127.0.0.1 --tcp-port 8081 --duration 15s --concurrency 8
./bin/openbyte-load --mode tcp-upload --host 127.0.0.1 --tcp-port 8081 --duration 15s --concurrency 8
./bin/openbyte-load --mode udp-download --host 127.0.0.1 --udp-port 8082 --duration 10s --concurrency 4 --packet-size 1200
./bin/openbyte-load --mode udp-upload --host 127.0.0.1 --udp-port 8082 --duration 10s --concurrency 4 --packet-size 1200
```

### Suggested perf scenarios

- TCP download: `duration=30s`, `concurrency=8`, MTU-sized chunks
- TCP upload: `duration=30s`, `concurrency=8`
- UDP download/upload: `duration=20s`, `concurrency=4`, `packet-size=1200`
- WebSocket fanout: use `--mode ws --ws-url ws://host/api/v1/stream/<id>/stream` with `concurrency=100+`
