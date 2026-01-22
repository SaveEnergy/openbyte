.PHONY: build server client loadtest test clean run help docker docker-up docker-down perf-smoke perf-bench ci-test ci-lint

# Build targets
build: server client

server:
	@echo "Building server..."
	@mkdir -p bin
	@go build -o bin/openbyte ./cmd/server
	@echo "✓ Server built: ./bin/openbyte"

client:
	@echo "Building CLI client..."
	@mkdir -p bin
	@go build -o bin/obyte ./cmd/client
	@echo "✓ CLI built: ./bin/obyte"

loadtest:
	@echo "Building load test tool..."
	@mkdir -p bin
	@go build -o bin/obyte-load ./cmd/loadtest
	@echo "✓ Load test tool built: ./bin/obyte-load"

# Testing
test:
	@echo "Running tests..."
	@go test ./... -short -v

ci-test:
	@echo "Running CI tests..."
	@go test ./... -short

ci-lint:
	@echo "Running CI lint..."
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	@go vet ./...

test-e2e:
	@echo "Running e2e tests..."
	@go test ./test/e2e -v -timeout 30s

test-race:
	@echo "Running tests with race detector..."
	@go test ./... -race -short

test-coverage:
	@echo "Generating test coverage..."
	@go test ./... -short -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

perf-bench:
	@echo "Running perf benchmarks..."
	@go test ./internal/metrics -run Test -bench . -benchtime=1s
	@go test ./internal/websocket -run Test -bench . -benchtime=1s
	@go test ./internal/stream -run Test -bench . -benchtime=1s

# Development
run:
	@echo "Starting server..."
	@echo "Port: $${PORT:-8080} (set PORT env var to change)"
	@go run ./cmd/server

run-quic:
	@echo "Starting server with QUIC enabled..."
	@echo "Ports: HTTP=8080, TCP=8081, UDP=8082, QUIC=8083"
	@QUIC_ENABLED=true go run ./cmd/server

run-alt-ports:
	@echo "Starting server with alternative ports..."
	@echo "HTTP: 9090, TCP test: 9081, UDP test: 9082"
	@PORT=9090 TCP_TEST_PORT=9081 UDP_TEST_PORT=9082 go run ./cmd/server

kill-ports:
	@echo "Killing processes on ports 8080, 8081, 8082, 8083..."
	@-lsof -ti :8080 2>/dev/null | xargs kill -9 2>/dev/null || true
	@-lsof -ti :8081 2>/dev/null | xargs kill -9 2>/dev/null || true
	@-lsof -ti :8082 2>/dev/null | xargs kill -9 2>/dev/null || true
	@-lsof -ti :8083 2>/dev/null | xargs kill -9 2>/dev/null || true
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
	@rm -f openbyte obyte obyte-load coverage.out coverage.html
	@rm -rf bin/
	@echo "✓ Cleaned"

perf-smoke: loadtest
	@echo "Starting server with pprof..."
	@PPROF_ENABLED=true PPROF_ADDR=127.0.0.1:6060 PORT=8080 TCP_TEST_PORT=8081 UDP_TEST_PORT=8082 go run ./cmd/server & echo $$! > /tmp/openbyte-perf.pid
	@sleep 2
	@$(MAKE) perf-bench
	@./bin/obyte-load --mode tcp-download --host 127.0.0.1 --tcp-port 8081 --duration 5s --concurrency 4
	@curl -s "http://127.0.0.1:6060/debug/pprof/profile?seconds=5" -o /tmp/openbyte-cpu.pprof
	@kill `cat /tmp/openbyte-perf.pid` || true
	@echo "✓ Perf smoke complete. Profile: /tmp/openbyte-cpu.pprof"

# Help
help:
	@echo "openByte Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build         - Build server and client"
	@echo "  server        - Build server binary (openbyte)"
	@echo "  client        - Build CLI client (obyte)"
	@echo "  loadtest      - Build load test tool (obyte-load)"
	@echo "  test          - Run tests"
	@echo "  ci-test       - Run CI test suite (short)"
	@echo "  ci-lint       - Run CI lint checks"
	@echo "  test-race     - Run tests with race detector"
	@echo "  test-coverage - Generate coverage report"
	@echo "  perf-bench    - Run perf benchmarks"
	@echo "  perf-smoke    - Run perf smoke with pprof capture"
	@echo "  run           - Run server (development, port 8080)"
	@echo "  run-quic      - Run server with QUIC enabled (ports 8080-8083)"
	@echo "  run-alt-ports - Run server with alternative ports (9090, 9081, 9082)"
	@echo "  kill-ports    - Kill processes on ports 8080-8083"
	@echo "  docker        - Build Docker image"
	@echo "  docker-up     - Start Docker containers"
	@echo "  docker-down   - Stop Docker containers"
	@echo "  clean         - Remove build artifacts"
	@echo "  help          - Show this help"

