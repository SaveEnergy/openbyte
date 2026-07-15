.PHONY: build openbyte test test-ui test-e2e test-race test-coverage clean run help docker docker-up docker-down perf-smoke perf-bench perf-leakcheck ci-lint lint-openapi

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Build targets
build: openbyte

openbyte:
	@echo "Building openbyte..."
	@mkdir -p bin
	@go build -ldflags "$(LDFLAGS)" -o bin/openbyte ./cmd/openbyte
	@echo "✓ Binary built: ./bin/openbyte"


# Testing
test:
	@echo "Running tests..."
	@go test ./... -short -v

test-ui:
	@echo "Running Playwright UI tests..."
	@env -u NO_COLOR bun run test:ui

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

perf-bench:
	@BENCH_COUNT=$${BENCH_COUNT:-1} BENCH_TIME=$${BENCH_TIME:-1s} "$(CURDIR)/scripts/perf/run_benchmarks.sh"

# Development
run:
	@echo "Starting server..."
	@echo "Port: $${PORT:-8080} (set PORT env var to change)"
	@go run -ldflags "$(LDFLAGS)" ./cmd/openbyte


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
	@rm -f openbyte coverage.out coverage.html
	@rm -rf bin/
	@echo "✓ Cleaned"

perf-smoke: build
	@echo "Starting server with pprof..."
	@PPROF_ENABLED=true PPROF_ADDR=127.0.0.1:6060 PORT=8080 ./bin/openbyte & echo $$! > /tmp/openbyte-perf.pid
	@sleep 2
	@$(MAKE) perf-bench
	@curl -sf "http://127.0.0.1:8080/api/v1/ping" >/dev/null
	@curl -s "http://127.0.0.1:6060/debug/pprof/profile?seconds=5" -o /tmp/openbyte-cpu.pprof
	@kill `cat /tmp/openbyte-perf.pid` || true
	@echo "✓ Perf smoke complete. Profile: /tmp/openbyte-cpu.pprof"

perf-leakcheck:
	@echo "Building server with goroutine leak profile experiment..."
	@mkdir -p bin
	@GOEXPERIMENT=goroutineleakprofile go build -ldflags "$(LDFLAGS)" -o bin/openbyte-leak ./cmd/openbyte
	@echo "Starting leak-profile server with pprof..."
	@PPROF_ENABLED=true PPROF_ADDR=127.0.0.1:6061 PORT=8090 ./bin/openbyte-leak server & echo $$! > /tmp/openbyte-leak.pid
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
	@echo "  test          - Run short Go test suite"
	@echo "  test-ui       - Run Playwright UI tests"
	@echo "  test-e2e      - Run full Go E2E tests"
	@echo "  ci-lint       - Run CI lint checks"
	@echo "  lint-openapi  - Lint api/openapi.yaml (Bun + devDependencies)"
	@echo "  test-race     - Run short Go suite with race detector"
	@echo "  test-coverage - Generate short-suite coverage report"
	@echo "  perf-bench    - Run perf benchmarks (stdout; quick count)"
	@echo "  perf-smoke    - Run perf smoke with pprof capture"
	@echo "  perf-leakcheck - Run goroutine leak profile smoke (Go 1.26 experiment)"
	@echo "  run           - Run server (development, port 8080)"
	@echo "  docker        - Build Docker image"
	@echo "  docker-up     - Start Docker containers"
	@echo "  docker-down   - Stop Docker containers"
	@echo "  clean         - Remove build artifacts"
	@echo "  help          - Show this help"
