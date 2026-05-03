// Package check implements the `openbyte check` subcommand — a quick 3-5s
// connectivity test returning grade, summary, and key metrics.
package check

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/saveenergy/openbyte/pkg/client"
	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

var (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
	runCheckFn  = runCheck
)

const (
	minTimeoutSeconds = 1
	maxTimeoutSeconds = 300
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
	)
	flagSet.StringVar(&serverURL, "server-url", "http://localhost:8080", "Server URL")
	flagSet.StringVar(&serverURL, "S", "http://localhost:8080", "Server URL (short)")
	flagSet.BoolVar(&jsonOut, "json", false, "Output as JSON")
	flagSet.IntVar(&timeout, "timeout", 10, "Overall timeout in seconds")
	help := flagSet.Bool("help", false, "Show help")
	flagSet.BoolVar(help, "h", false, "Show help (short)")
	versionFlag := flagSet.Bool("version", false, "Print version")

	if flagSet.Parse(args) != nil {
		return exitUsage
	}

	if *help {
		printUsage()
		return exitSuccess
	}
	if *versionFlag {
		fmt.Printf("openbyte %s\n", version)
		return exitSuccess
	}

	resolvedURL, usageErr := resolveCheckServerURL(serverURL, flagSet.Args(), timeout)
	if usageErr != nil {
		fmt.Fprintln(os.Stderr, usageErr)
		return exitUsage
	}
	serverURL = resolvedURL

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	result, err := runCheckFn(ctx, serverURL)

	if err != nil {
		writeCheckError(jsonOut, err)
		return exitFailure
	}
	if writeCheckResult(jsonOut, result) != nil {
		return exitFailure
	}

	// Exit 1 if grade is D or F (degraded)
	if result.Interpretation != nil && (result.Interpretation.Grade == "D" || result.Interpretation.Grade == "F") {
		return exitFailure
	}
	return exitSuccess
}

func resolveCheckServerURL(currentURL string, rest []string, timeout int) (string, error) {
	if timeout < minTimeoutSeconds || timeout > maxTimeoutSeconds {
		return "", fmt.Errorf("openbyte check: timeout must be between %d and %d seconds", minTimeoutSeconds, maxTimeoutSeconds)
	}
	if len(rest) > 1 {
		return "", fmt.Errorf("openbyte check: too many positional arguments")
	}
	if len(rest) == 1 {
		arg := rest[0]
		if !isValidServerURL(arg) {
			return "", fmt.Errorf("openbyte check: invalid server URL: %q", arg)
		}
		currentURL = arg
	}
	if !isValidServerURL(currentURL) {
		return "", fmt.Errorf("openbyte check: invalid server URL: %q", currentURL)
	}
	return currentURL, nil
}

func writeCheckError(jsonOut bool, err error) {
	if jsonOut {
		errResp := map[string]any{
			"schema_version": "1.0",
			"error":          true,
			"code":           "check_failed",
			"message":        err.Error(),
		}
		if encErr := json.NewEncoder(os.Stdout).Encode(errResp); encErr != nil {
			fmt.Fprintf(os.Stderr, "openbyte check: json encode error: %v\n", encErr)
		}
		return
	}
	fmt.Fprintf(os.Stderr, "openbyte check: error: %v\n", err)
}

func writeCheckResult(jsonOut bool, result *CheckResult) error {
	if jsonOut {
		if encErr := json.NewEncoder(os.Stdout).Encode(result); encErr != nil {
			fmt.Fprintf(os.Stderr, "openbyte check: json encode error: %v\n", encErr)
			return encErr
		}
		return nil
	}
	printHuman(result)
	return nil
}

func runCheck(ctx context.Context, serverURL string) (*CheckResult, error) {
	c := client.New(serverURL)
	r, err := c.Check(ctx)
	if err != nil {
		return nil, err
	}
	return &CheckResult{
		SchemaVersion:  "1.0",
		Status:         r.Status,
		ServerURL:      r.ServerURL,
		LatencyMs:      r.LatencyMs,
		DownloadMbps:   r.DownloadMbps,
		UploadMbps:     r.UploadMbps,
		JitterMs:       r.JitterMs,
		Interpretation: r.Interpretation,
		DurationMs:     r.DurationMs,
	}, nil
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
  --version               Print version
  -S, --server-url string Server URL (default: http://localhost:8080)
  --json                  Output as JSON
  --timeout int           Overall timeout in seconds (default: 10)
Exit codes:
  0   Healthy (grade A-C)
  1   Degraded (grade D-F) or error

Examples:
  openbyte check                              # Quick check against localhost
  openbyte check https://speed.example.com    # Quick check against remote
  openbyte check --json                       # JSON output for agents
`)
}

func isValidServerURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u == nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if port := u.Port(); port != "" {
		n, convErr := strconv.Atoi(port)
		if convErr != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return true
}
