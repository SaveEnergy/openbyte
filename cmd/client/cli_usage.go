package client

import (
	"fmt"
	"os"
)

func printUsage() {
	fmt.Fprintf(os.Stdout, `Usage: openbyte client [flags] [server-url]

Run network speed test (client-side measurement).

Server:
  openbyte client <url>             Use server URL directly
  openbyte client -S <url>          Use server URL directly

Flags:
  -h, --help              Show help
  --version               Print version
  -S string               Server URL (short)
  -d, --direction string  Direction: download or upload (default: download)
  -t, --duration int      Test duration in seconds (1-300) (default: 30)
  -s, --streams int       Parallel streams (1-64) (default: 4)
  --chunk-size int        HTTP chunk size in bytes (65536-4194304) (default: 1048576)
  --json                  Output results as JSON
  --ndjson                Streaming newline-delimited JSON (progress + result)
  --plain                 Plain text output
  -v, --verbose           Verbose output
  -q, --quiet             Quiet mode (errors only)
  --no-color              Disable color output
  --no-progress           Disable progress indicators
  --timeout int           Request timeout in seconds (default: 60)
  --server-url string     Override server URL

Measurement:
  --warmup int            Warm-up seconds before measurement (default: 2)

Configuration file: ~/.config/openbyte/config.yaml

Environment:
  NO_COLOR                Disable colors (standard convention)

Examples:
  openbyte client                          # Default test
  openbyte client https://speed.example.com
  openbyte client -d upload -t 60          # Upload test, 60s
  openbyte client --json https://speed.example.com
`)
}
