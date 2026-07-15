.PHONY: build test test-ui test-race test-coverage clean run help perf-bench ci-lint lint-openapi

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Build targets
build:
	@echo "Building openbyte..."
	@mkdir -p bin
	@go build -ldflags "$(LDFLAGS)" -o bin/openbyte ./cmd/openbyte
	@echo "✓ Binary built: ./bin/openbyte"


# Testing
test:
	@echo "Running tests..."
	@go test ./... -v

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

test-race:
	@echo "Running tests with race detector..."
	@go test ./... -race

test-coverage:
	@echo "Generating test coverage..."
	@go test ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

perf-bench:
	@BENCH_COUNT=$${BENCH_COUNT:-1} BENCH_TIME=$${BENCH_TIME:-1s} "$(CURDIR)/scripts/perf/run_benchmarks.sh"

# Development
run:
	@echo "Starting server..."
	@echo "Port: $${PORT:-8080} (set PORT env var to change)"
	@go run -ldflags "$(LDFLAGS)" ./cmd/openbyte

# Cleanup
clean:
	@echo "Cleaning build artifacts..."
	@rm -f coverage.out coverage.html
	@rm -rf bin/
	@echo "✓ Cleaned"

# Help
help:
	@echo "openByte Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build         - Build openbyte binary"
	@echo "  test          - Run Go test suite"
	@echo "  test-ui       - Run Playwright UI tests"
	@echo "  ci-lint       - Run CI lint checks"
	@echo "  lint-openapi  - Lint api/openapi.yaml (Bun + devDependencies)"
	@echo "  test-race     - Run Go suite with race detector"
	@echo "  test-coverage - Generate Go test coverage report"
	@echo "  perf-bench    - Run perf benchmarks (stdout; quick count)"
	@echo "  run           - Run server (development, port 8080)"
	@echo "  clean         - Remove build artifacts"
	@echo "  help          - Show this help"
