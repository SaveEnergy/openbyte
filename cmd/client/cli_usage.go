package client

import (
	"fmt"
	"os"
)

func printUsage() {
	fmt.Fprintf(os.Stdout, `Usage: openbyte client [flags] [server]

Run network speed test (client-side measurement).

Server Selection:
  openbyte client <alias>           Use server alias from config
  openbyte client <url>             Use server URL directly
  openbyte client -S <url>          Use server URL directly
  openbyte client -a, --auto        Auto-select fastest server
  openbyte client --servers         List configured servers

Flags:
  -h, --help              Show help
  --version               Print version
  -a, --auto              Auto-select fastest server (lowest latency)
  -S string               Server URL (short)
  --server string         Server alias
  --servers               List configured servers
  -p, --protocol string   Protocol: tcp, udp, http (default: tcp)
  -d, --direction string  Direction: download, upload, bidirectional (default: download)
  -t, --duration int      Test duration in seconds (1-300) (default: 30)
  -s, --streams int       Parallel streams (1-64) (default: 4)
  --packet-size int       Packet size in bytes (64-9000) (default: 1400)
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
  openbyte client -a                       # Auto-select fastest server
  openbyte client -d upload -t 60          # Upload test, 60s
  openbyte client -p udp -d bidirectional -s 8
  openbyte client --json server.example.com
`)
}
