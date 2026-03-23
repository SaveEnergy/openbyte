.PHONY: build openbyte loadtest test test-ui clean run help docker docker-up docker-down perf-smoke perf-bench perf-record perf-compare perf-check perf-leakcheck autoresearch-preflight autoresearch-loop-complete ci-test ci-lint lint-openapi

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Build targets
build: openbyte

openbyte:
	@echo "Building openbyte..."
	@mkdir -p bin
	@go build -ldflags "$(LDFLAGS)" -o bin/openbyte ./cmd/openbyte
	@echo "✓ Binary built: ./bin/openbyte"

loadtest:
	@echo "Building load test tool..."
	@mkdir -p bin
	@go build -ldflags "$(LDFLAGS)" -o bin/openbyte-load ./cmd/loadtest
	@echo "✓ Load test tool built: ./bin/openbyte-load"

# Testing
test:
	@echo "Running tests..."
	@go test ./... -short -v

test-ui:
	@echo "Running Playwright UI tests..."
	@env -u NO_COLOR bunx playwright test

ci-test:
	@echo "Running CI tests..."
	@go test ./... -short

ci-lint:
	@echo "Running CI lint..."
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	@go vet ./...

# Requires Bun + `bun install` (same as CI `bun run lint:openapi`)
lint-openapi:
	@bun run lint:openapi

test-e2e:
	@echo "Running e2e tests..."
	@go test ./test/e2e -v -timeout 2m

test-race:
	@echo "Running tests with race detector..."
	@go test ./... -race -short

test-coverage:
	@echo "Generating test coverage..."
	@go test ./... -short -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

# Unified suite: test/perf/bench_packages.txt + scripts/perf/run_benchmarks.sh
perf-bench:
	@BENCH_COUNT=$${BENCH_COUNT:-1} BENCH_TIME=$${BENCH_TIME:-1s} "$(CURDIR)/scripts/perf/run_benchmarks.sh" --stdout

perf-record:
	@BENCH_COUNT=$${BENCH_COUNT:-5} BENCH_TIME=$${BENCH_TIME:-1s} "$(CURDIR)/scripts/perf/run_benchmarks.sh"

# Compare build/perf/bench.txt to test/perf/bench_baseline.txt (uses benchstat on PATH, else go run …@latest)
perf-compare:
	@test -f test/perf/bench_baseline.txt || (echo "Missing test/perf/bench_baseline.txt — run: make perf-record && cp build/perf/bench.txt test/perf/bench_baseline.txt"; exit 1)
	@test -f build/perf/bench.txt || (echo "Missing build/perf/bench.txt — run make perf-record first"; exit 1)
	@if command -v benchstat >/dev/null 2>&1; then \
		benchstat test/perf/bench_baseline.txt build/perf/bench.txt; \
	else \
		echo "perf-compare: benchstat not on PATH; using go run golang.org/x/perf/cmd/benchstat@latest"; \
		go run golang.org/x/perf/cmd/benchstat@latest test/perf/bench_baseline.txt build/perf/bench.txt; \
	fi

autoresearch-preflight:
	@bash "$(CURDIR)/scripts/perf/autoresearch_preflight.sh"

autoresearch-loop-complete:
	@bash "$(CURDIR)/scripts/perf/autoresearch_loop_complete.sh"

perf-check: perf-record
	@if [ -f test/perf/bench_baseline.txt ]; then $(MAKE) perf-compare; else echo "perf-check: wrote build/perf/bench.txt (add test/perf/bench_baseline.txt to enable compare)"; fi

# Development
run:
	@echo "Starting server..."
	@echo "Port: $${PORT:-8080} (set PORT env var to change)"
	@go run -ldflags "$(LDFLAGS)" ./cmd/openbyte server

run-alt-ports:
	@echo "Starting server with alternative ports..."
	@echo "HTTP: 9090, TCP test: 9081, UDP test: 9082"
	@PORT=9090 TCP_TEST_PORT=9081 UDP_TEST_PORT=9082 go run -ldflags "$(LDFLAGS)" ./cmd/openbyte server

kill-ports:
	@echo "Killing processes on ports 8080, 8081, 8082..."
	@-lsof -ti :8080 2>/dev/null | xargs kill -9 2>/dev/null || true
	@-lsof -ti :8081 2>/dev/null | xargs kill -9 2>/dev/null || true
	@-lsof -ti :8082 2>/dev/null | xargs kill -9 2>/dev/null || true
	@sleep 1
	@echo "✓ Ports cleared"

# Docker
docker:
	@echo "Building Docker image..."
	@docker build -f docker/Dockerfile -t openbyte:latest --target server .
	@echo "✓ Docker image built: openbyte:latest"

docker-up:
	@echo "Starting Docker containers..."
	@cd docker && docker compose up -d
	@echo "✓ Containers started"

docker-down:
	@echo "Stopping Docker containers..."
	@cd docker && docker compose down
	@echo "✓ Containers stopped"

# Cleanup
clean:
	@echo "Cleaning build artifacts..."
	@rm -f openbyte openbyte-load coverage.out coverage.html
	@rm -rf bin/
	@echo "✓ Cleaned"

perf-smoke: build loadtest
	@echo "Starting server with pprof..."
	@PPROF_ENABLED=true PPROF_ADDR=127.0.0.1:6060 PORT=8080 TCP_TEST_PORT=8081 UDP_TEST_PORT=8082 ./bin/openbyte server & echo $$! > /tmp/openbyte-perf.pid
	@sleep 2
	@$(MAKE) perf-bench
	@./bin/openbyte-load --mode tcp-download --host 127.0.0.1 --tcp-port 8081 --duration 5s --concurrency 4
	@curl -s "http://127.0.0.1:6060/debug/pprof/profile?seconds=5" -o /tmp/openbyte-cpu.pprof
	@kill `cat /tmp/openbyte-perf.pid` || true
	@echo "✓ Perf smoke complete. Profile: /tmp/openbyte-cpu.pprof"

perf-leakcheck:
	@echo "Building server with goroutine leak profile experiment..."
	@mkdir -p bin
	@GOEXPERIMENT=goroutineleakprofile go build -ldflags "$(LDFLAGS)" -o bin/openbyte-leak ./cmd/openbyte
	@echo "Starting leak-profile server with pprof..."
	@PPROF_ENABLED=true PPROF_ADDR=127.0.0.1:6061 PORT=8090 TCP_TEST_PORT=8091 UDP_TEST_PORT=8092 ./bin/openbyte-leak server & echo $$! > /tmp/openbyte-leak.pid
	@sleep 2
	@curl -sf "http://127.0.0.1:6061/debug/pprof/goroutineleak?debug=1" -o /tmp/openbyte-goroutineleak.txt
	@kill `cat /tmp/openbyte-leak.pid` || true
	@echo "✓ Goroutine leak profile captured: /tmp/openbyte-goroutineleak.txt"

# Help
help:
	@echo "openByte Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build         - Build openbyte binary"
	@echo "  openbyte      - Build openbyte binary"
	@echo "  loadtest      - Build load test tool (openbyte-load)"
	@echo "  test          - Run tests"
	@echo "  test-ui       - Run Playwright UI tests"
	@echo "  ci-test       - Run CI test suite (short)"
	@echo "  ci-lint       - Run CI lint checks"
	@echo "  lint-openapi  - Lint api/openapi.yaml (Bun + devDependencies)"
	@echo "  test-race     - Run tests with race detector"
	@echo "  test-coverage - Generate coverage report"
	@echo "  perf-bench    - Run perf benchmarks (stdout; quick count)"
	@echo "  perf-record   - Write build/perf/bench.txt (stable; for benchstat)"
	@echo "  perf-compare  - benchstat baseline vs build/perf/bench.txt (go run fallback)"
	@echo "  perf-check    - perf-record + perf-compare if baseline exists"
	@echo "  autoresearch-preflight - verify counter/branch + print AUTORESEARCH_* (perf agents)"
	@echo "  autoresearch-loop-complete - merge perf-N into main, bump counter, start perf-(N+1) (loop mode)"
	@echo "  perf-smoke    - Run perf smoke with pprof capture"
	@echo "  perf-leakcheck - Run goroutine leak profile smoke (Go 1.26 experiment)"
	@echo "  run           - Run server (development, port 8080)"
	@echo "  run-alt-ports - Run server with alternative ports (9090, 9081, 9082)"
	@echo "  kill-ports    - Kill processes on ports 8080-8082"
	@echo "  docker        - Build Docker image"
	@echo "  docker-up     - Start Docker containers"
	@echo "  docker-down   - Stop Docker containers"
	@echo "  clean         - Remove build artifacts"
	@echo "  help          - Show this help"

