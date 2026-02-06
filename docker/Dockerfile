# Build stage (base)
FROM golang:1.25-alpine AS builder-base

WORKDIR /app

# Build metadata
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source (web/ needed for //go:embed)
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY web/ ./web/

# Build openbyte binary
FROM builder-base AS builder-openbyte
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /openbyte ./cmd/openbyte

# Server image
FROM alpine:3.21 AS server

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy openbyte binary (web assets are embedded)
COPY --from=builder-openbyte /openbyte /app/openbyte

# Create non-root user and data directory
RUN adduser -D -H -s /sbin/nologin openbyte && \
    mkdir -p /app/data && chown openbyte:openbyte /app/data
USER openbyte

VOLUME /app/data

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
    RATE_LIMIT_PER_IP=100 \
    DATA_DIR=/app/data

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/openbyte", "server"]

# Client image
FROM alpine:3.21 AS client

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy openbyte binary
COPY --from=builder-openbyte /openbyte /app/openbyte

# Create non-root user
RUN adduser -D -H -s /sbin/nologin openbyte
USER openbyte

ENTRYPOINT ["/app/openbyte", "client"]
CMD ["--help"]
