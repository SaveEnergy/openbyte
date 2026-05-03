// Package client provides a Go SDK for running openByte speed tests
// programmatically. Agents and applications can import this package instead
// of shelling out to the CLI.
//
// Usage:
//
//	c := client.New("https://speedtest.example.com")
//	result, err := c.Check(ctx)
//	result, err := c.SpeedTest(ctx, client.SpeedTestOptions{...})
package client

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	directionDownload = "download"
	directionUpload   = "upload"
	pathHealth        = "/health"
	pathPing          = "/api/v1/ping"
	pathDownload      = "/api/v1/download"
	pathUpload        = "/api/v1/upload"
)

var (
	ErrLatencyMeasurementFailed  = errors.New("latency measurement failed")
	ErrDownloadMeasurementFailed = errors.New("download measurement failed")
	ErrUploadMeasurementFailed   = errors.New("upload measurement failed")
)

// Client is an openByte speed test client targeting a single server.
// It is safe for concurrent use as long as options are not mutated after New().
type Client struct {
	serverURL  string
	httpClient *http.Client
}

// Option configures the Client.
type Option func(*Client)

// WithHTTPClient overrides the default http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

const defaultHTTPTimeout = 60 * time.Second

// New creates a new openByte client targeting the given server URL.
// Returned client should be treated as immutable after construction.
// Default http.Client has a 60s timeout to avoid indefinite hangs on stalled connections.
func New(serverURL string, opts ...Option) *Client {
	c := &Client{
		serverURL: strings.TrimRight(serverURL, "/"),
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
