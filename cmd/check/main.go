// Package check implements the `openbyte check` subcommand — a quick 3-5s
// connectivity test returning grade, summary, and key metrics.
package check

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

var (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

// CheckResult is the structured output of openbyte check.
type CheckResult struct {
	SchemaVersion  string                     `json:"schema_version"`
	Status         string                     `json:"status"`
	ServerURL      string                     `json:"server_url"`
	LatencyMs      float64                    `json:"latency_ms"`
	DownloadMbps   float64                    `json:"download_mbps"`
	UploadMbps     float64                    `json:"upload_mbps"`
	JitterMs       float64                    `json:"jitter_ms"`
	Interpretation *diagnostic.Interpretation `json:"interpretation"`
	DurationMs     int64                      `json:"duration_ms"`
}

func Run(args []string, version string) int {
	flagSet := flag.NewFlagSet("openbyte check", flag.ContinueOnError)
	flagSet.SetOutput(os.Stdout)

	var (
		serverURL string
		jsonOut   bool
		timeout   int
		apiKey    string
	)
	flagSet.StringVar(&serverURL, "server-url", "http://localhost:8080", "Server URL")
	flagSet.StringVar(&serverURL, "S", "http://localhost:8080", "Server URL (short)")
	flagSet.BoolVar(&jsonOut, "json", false, "Output as JSON")
	flagSet.IntVar(&timeout, "timeout", 10, "Overall timeout in seconds")
	flagSet.StringVar(&apiKey, "api-key", "", "API key")
	help := flagSet.Bool("help", false, "Show help")
	flagSet.BoolVar(help, "h", false, "Show help (short)")

	if err := flagSet.Parse(args); err != nil {
		return exitUsage
	}

	if *help {
		printUsage()
		return exitSuccess
	}

	// Positional arg = server URL
	rest := flagSet.Args()
	if len(rest) > 0 {
		arg := rest[0]
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			serverURL = arg
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	start := time.Now()
	result, err := runCheck(ctx, serverURL, apiKey)
	result.DurationMs = time.Since(start).Milliseconds()

	if err != nil {
		if jsonOut {
			errResp := map[string]interface{}{
				"schema_version": "1.0",
				"error":          true,
				"code":           "check_failed",
				"message":        err.Error(),
			}
			json.NewEncoder(os.Stdout).Encode(errResp)
		} else {
			fmt.Fprintf(os.Stderr, "openbyte check: error: %v\n", err)
		}
		return exitFailure
	}

	if jsonOut {
		json.NewEncoder(os.Stdout).Encode(result)
	} else {
		printHuman(result)
	}

	// Exit 1 if grade is D or F (degraded)
	if result.Interpretation != nil && (result.Interpretation.Grade == "D" || result.Interpretation.Grade == "F") {
		return exitFailure
	}
	return exitSuccess
}

func runCheck(ctx context.Context, serverURL, apiKey string) (*CheckResult, error) {
	base := strings.TrimRight(serverURL, "/")
	client := &http.Client{Timeout: 5 * time.Second}

	// Phase 1: Health check
	healthURL := base + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return nil, fmt.Errorf("server unreachable: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("server unreachable: %w", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server unhealthy: status %d", resp.StatusCode)
	}

	// Phase 2: Latency (5 pings)
	pingURL := base + "/api/v1/ping"
	var latencies []time.Duration
	for i := 0; i < 5; i++ {
		if ctx.Err() != nil {
			break
		}
		pStart := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
		if err != nil {
			continue
		}
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		latencies = append(latencies, time.Since(pStart))
	}

	var avgLatencyMs float64
	var jitterMs float64
	if len(latencies) > 0 {
		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		avgLatencyMs = float64(total) / float64(len(latencies)) / float64(time.Millisecond)

		if len(latencies) >= 2 {
			var jitterSum float64
			for i := 1; i < len(latencies); i++ {
				diff := latencies[i] - latencies[i-1]
				if diff < 0 {
					diff = -diff
				}
				jitterSum += float64(diff) / float64(time.Millisecond)
			}
			jitterMs = jitterSum / float64(len(latencies)-1)
		}
	}

	// Phase 3: Quick download burst (1s)
	downMbps := quickDownload(ctx, base, apiKey, client)

	// Phase 4: Quick upload burst (1s)
	upMbps := quickUpload(ctx, base, apiKey, client)

	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: downMbps,
		UploadMbps:   upMbps,
		LatencyMs:    avgLatencyMs,
		JitterMs:     jitterMs,
		PacketLoss:   0,
	})

	return &CheckResult{
		SchemaVersion:  "1.0",
		Status:         "ok",
		ServerURL:      serverURL,
		LatencyMs:      avgLatencyMs,
		DownloadMbps:   downMbps,
		UploadMbps:     upMbps,
		JitterMs:       jitterMs,
		Interpretation: interp,
	}, nil
}

func quickDownload(ctx context.Context, base, apiKey string, client *http.Client) float64 {
	dlCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	reqURL := base + "/api/v1/download?duration=2&chunk=1048576"
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("Accept-Encoding", "identity")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return 0
	}

	buf := make([]byte, 64*1024)
	var totalBytes int64
	for {
		n, readErr := resp.Body.Read(buf)
		totalBytes += int64(n)
		if readErr != nil {
			break
		}
	}

	elapsed := time.Since(start)
	if elapsed <= 0 {
		return 0
	}
	return float64(totalBytes*8) / elapsed.Seconds() / 1_000_000
}

func quickUpload(ctx context.Context, base, apiKey string, client *http.Client) float64 {
	upCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Upload 1MB payload
	payload := make([]byte, 1024*1024)
	reqURL := base + "/api/v1/upload"
	req, err := http.NewRequestWithContext(upCtx, http.MethodPost, reqURL, strings.NewReader(string(payload)))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	elapsed := time.Since(start)
	if elapsed <= 0 || resp.StatusCode != http.StatusOK {
		return 0
	}
	return float64(len(payload)*8) / elapsed.Seconds() / 1_000_000
}

func printHuman(r *CheckResult) {
	if r.Interpretation != nil {
		fmt.Printf("Grade: %s — %s\n", r.Interpretation.Grade, r.Interpretation.Summary)
	}
	fmt.Printf("  Latency:  %.1f ms\n", r.LatencyMs)
	fmt.Printf("  Download: %.1f Mbps\n", r.DownloadMbps)
	fmt.Printf("  Upload:   %.1f Mbps\n", r.UploadMbps)
	fmt.Printf("  Jitter:   %.1f ms\n", r.JitterMs)
	if r.Interpretation != nil && len(r.Interpretation.Concerns) > 0 {
		fmt.Printf("  Concerns: %s\n", strings.Join(r.Interpretation.Concerns, ", "))
	}
}

func printUsage() {
	fmt.Fprintf(os.Stdout, `Usage: openbyte check [flags] [server-url]

Quick connectivity check (~3-5 seconds). Returns grade + key metrics.

Flags:
  -h, --help              Show help
  -S, --server-url string Server URL (default: http://localhost:8080)
  --json                  Output as JSON
  --timeout int           Overall timeout in seconds (default: 10)
  --api-key string        API key for authentication

Exit codes:
  0   Healthy (grade A-C)
  1   Degraded (grade D-F) or error

Examples:
  openbyte check                              # Quick check against localhost
  openbyte check https://speed.example.com    # Quick check against remote
  openbyte check --json                       # JSON output for agents
`)
}
