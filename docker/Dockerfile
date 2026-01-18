# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build server binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /openbyte ./cmd/server

# Build client binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /obyte ./cmd/client

# Server image
FROM alpine:3.19 AS server

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

# Copy server binary
COPY --from=builder /openbyte /app/openbyte

# Copy web assets
COPY web/ /app/web/

# Create non-root user
RUN adduser -D -H -s /sbin/nologin openbyte
USER openbyte

# Expose ports: API (8080), TCP test (8081), UDP test (8082)
EXPOSE 8080 8081 8082/tcp 8082/udp

# Environment variables for server identity
ENV PORT=8080 \
    BIND_ADDRESS=0.0.0.0 \
    TCP_TEST_PORT=8081 \
    UDP_TEST_PORT=8082 \
    SERVER_ID="" \
    SERVER_NAME="OpenByte Server" \
    SERVER_LOCATION="" \
    SERVER_REGION="" \
    PUBLIC_HOST="" \
    CAPACITY_GBPS=25 \
    MAX_CONCURRENT_TESTS=10 \
    MAX_STREAMS=16 \
    RATE_LIMIT_PER_IP=100

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/openbyte"]

# Client image
FROM alpine:3.19 AS client

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy client binary
COPY --from=builder /obyte /app/obyte

# Create non-root user
RUN adduser -D -H -s /sbin/nologin obyte
USER obyte

ENTRYPOINT ["/app/obyte"]
CMD ["--help"]
