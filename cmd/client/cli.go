package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func parseFlags() (*Config, map[string]bool) {
	config := &Config{}
	flagsSet := make(map[string]bool)

	flag.StringVar(&config.Protocol, "protocol", "", "Protocol: tcp, udp, quic")
	flag.StringVar(&config.Protocol, "p", "", "Protocol: tcp, udp, quic (short)")
	flag.StringVar(&config.Direction, "direction", "", "Direction: download, upload, bidirectional")
	flag.StringVar(&config.Direction, "d", "", "Direction: download, upload, bidirectional (short)")
	flag.IntVar(&config.Duration, "duration", 0, "Test duration in seconds (1-300)")
	flag.IntVar(&config.Duration, "t", 0, "Test duration in seconds (1-300) (short)")
	flag.IntVar(&config.Streams, "streams", 0, "Parallel streams (1-16)")
	flag.IntVar(&config.Streams, "s", 0, "Parallel streams (1-16) (short)")
	flag.IntVar(&config.PacketSize, "packet-size", 0, "Packet size in bytes (64-9000)")
	flag.BoolVar(&config.JSON, "json", false, "Output results as JSON")
	flag.BoolVar(&config.Plain, "plain", false, "Plain text output")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&config.Verbose, "v", false, "Verbose output (short)")
	flag.BoolVar(&config.Quiet, "quiet", false, "Quiet mode (errors only)")
	flag.BoolVar(&config.Quiet, "q", false, "Quiet mode (errors only) (short)")
	flag.BoolVar(&config.NoColor, "no-color", false, "Disable color output")
	flag.BoolVar(&config.NoProgress, "no-progress", false, "Disable progress indicators")
	flag.StringVar(&config.Server, "server", "", "Server alias or URL")
	flag.StringVar(&config.Server, "S", "", "Server alias or URL (short)")
	flag.StringVar(&config.ServerURL, "server-url", "", "Server URL (override)")
	flag.StringVar(&config.APIKey, "api-key", "", "API key for authentication")
	flag.IntVar(&config.Timeout, "timeout", 0, "Request timeout in seconds")

	flag.IntVar(&config.WarmUp, "warmup", 2, "Warm-up seconds before measurement")
	flag.BoolVar(&config.Auto, "auto", false, "Auto-select fastest server")
	flag.BoolVar(&config.Auto, "a", false, "Auto-select fastest server (short)")

	version := flag.Bool("version", false, "Print version")
	help := flag.Bool("help", false, "Show help")
	flag.BoolVar(help, "h", false, "Show help (short)")
	servers := flag.Bool("servers", false, "List configured servers")

	flag.Parse()

	flag.Visit(func(f *flag.Flag) {
		flagsSet[f.Name] = true
		switch f.Name {
		case "p":
			flagsSet["protocol"] = true
		case "d":
			flagsSet["direction"] = true
		case "t":
			flagsSet["duration"] = true
		case "s":
			flagsSet["streams"] = true
		case "S":
			flagsSet["server"] = true
		case "v":
			flagsSet["verbose"] = true
		case "q":
			flagsSet["quiet"] = true
		case "h":
			flagsSet["help"] = true
		case "a":
			flagsSet["auto"] = true
		}
	})

	if *servers {
		listServers()
		os.Exit(exitSuccess)
	}

	if *version {
		fmt.Println("obyte 0.2.0")
		os.Exit(exitSuccess)
	}

	if *help {
		printUsage()
		os.Exit(exitSuccess)
	}

	args := flag.Args()
	if len(args) > 0 {
		server := args[0]
		if strings.HasPrefix(server, "http://") || strings.HasPrefix(server, "https://") {
			config.ServerURL = server
			flagsSet["server-url"] = true
		} else {
			config.Server = server
			flagsSet["server"] = true
		}
	}

	return config, flagsSet
}

func listServers() {
	configFile, err := loadConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "obyte: warning: %v\n", err)
	}

	fmt.Println("Configured Servers:")
	fmt.Println()

	if configFile == nil || len(configFile.Servers) == 0 {
		fmt.Println("  No servers configured.")
		fmt.Println()
		fmt.Println("Add servers to ~/.config/obyte/config.yaml:")
		fmt.Println()
		fmt.Println("  servers:")
		fmt.Println("    nyc:")
		fmt.Println("      url: https://speedtest-nyc.example.com")
		fmt.Println("      name: \"New York\"")
		fmt.Println("    ams:")
		fmt.Println("      url: https://speedtest-ams.example.com")
		fmt.Println("      name: \"Amsterdam\"")
		fmt.Println()
		return
	}

	fmt.Printf("  %-12s %-20s %s\n", "ALIAS", "NAME", "URL")
	fmt.Printf("  %-12s %-20s %s\n", "-----", "----", "---")
	for alias, server := range configFile.Servers {
		defaultMark := ""
		if alias == configFile.DefaultServer {
			defaultMark = " *"
		}
		name := server.Name
		if name == "" {
			name = alias
		}
		fmt.Printf("  %-12s %-20s %s%s\n", alias, name, server.URL, defaultMark)
	}
	fmt.Println()
	fmt.Println("  * = default server")
	fmt.Println()
	fmt.Println("Usage: obyte -S <alias> or obyte <alias>")
}

type ServerLatency struct {
	Alias   string
	URL     string
	Name    string
	Latency time.Duration
	Error   error
}

func selectFastestServer(configFile *ConfigFile, verbose bool) (*ServerLatency, error) {
	if configFile == nil || len(configFile.Servers) == 0 {
		return nil, fmt.Errorf("no servers configured for auto-selection")
	}

	results := make(chan ServerLatency, len(configFile.Servers))
	var wg sync.WaitGroup

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for alias, server := range configFile.Servers {
		wg.Add(1)
		go func(alias string, server ServerConfig) {
			defer wg.Done()
			result := ServerLatency{
				Alias: alias,
				URL:   server.URL,
				Name:  server.Name,
			}

			healthURL := strings.TrimSuffix(server.URL, "/") + "/health"
			start := time.Now()
			resp, err := client.Get(healthURL)
			result.Latency = time.Since(start)

			if err != nil {
				result.Error = err
			} else {
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					result.Error = fmt.Errorf("health check failed: %d", resp.StatusCode)
				}
			}

			results <- result
		}(alias, server)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var fastest *ServerLatency
	var allResults []ServerLatency

	for result := range results {
		allResults = append(allResults, result)
		if result.Error == nil {
			if fastest == nil || result.Latency < fastest.Latency {
				r := result
				fastest = &r
			}
		}
	}

	if verbose {
		fmt.Println("Server latencies:")
		for _, r := range allResults {
			status := fmt.Sprintf("%dms", r.Latency.Milliseconds())
			if r.Error != nil {
				status = "error"
			}
			marker := "  "
			if fastest != nil && r.Alias == fastest.Alias {
				marker = "→ "
			}
			name := r.Name
			if name == "" {
				name = r.Alias
			}
			fmt.Printf("%s%-12s %-20s %s\n", marker, r.Alias, name, status)
		}
		fmt.Println()
	}

	if fastest == nil {
		return nil, fmt.Errorf("all servers unreachable")
	}

	return fastest, nil
}

func validateConfig(config *Config) error {
	if config.Protocol != "tcp" && config.Protocol != "udp" && config.Protocol != "quic" {
		return fmt.Errorf("invalid protocol: %s\n\n"+
			"Protocol must be 'tcp', 'udp', or 'quic'.\n"+
			"Use: obyte -p tcp  or  obyte -p udp  or  obyte -p quic\n"+
			"See: obyte --help", config.Protocol)
	}
	if config.Direction != "download" && config.Direction != "upload" && config.Direction != "bidirectional" {
		return fmt.Errorf("invalid direction: %s\n\n"+
			"Direction must be 'download', 'upload', or 'bidirectional'.\n"+
			"Use: obyte -d download  or  obyte -d upload  or  obyte -d bidirectional\n"+
			"See: obyte --help", config.Direction)
	}
	if config.Duration < 1 || config.Duration > 300 {
		return fmt.Errorf("invalid duration: %d\n\n"+
			"Duration must be between 1 and 300 seconds.\n"+
			"Use: obyte -t 30  (for 30 seconds)\n"+
			"See: obyte --help", config.Duration)
	}
	if config.Streams < 1 || config.Streams > 16 {
		return fmt.Errorf("invalid streams: %d\n\n"+
			"Streams must be between 1 and 16.\n"+
			"Use: obyte -s 4  (for 4 parallel streams)\n"+
			"See: obyte --help", config.Streams)
	}
	if config.PacketSize < 64 || config.PacketSize > 9000 {
		return fmt.Errorf("invalid packet size: %d\n\n"+
			"Packet size must be between 64 and 9000 bytes.\n"+
			"Use: obyte --packet-size 1500  (for standard MTU)\n"+
			"See: obyte --help", config.PacketSize)
	}
	return nil
}

func printUsage() {
	fmt.Fprintf(os.Stdout, `Usage: obyte [flags] [server]

Run network speed test (client-side measurement).

Server Selection:
  obyte <alias>           Use server alias from config
  obyte <url>             Use server URL directly
  obyte -S <alias>        Select server by alias
  obyte -a, --auto        Auto-select fastest server
  obyte --servers         List configured servers

Flags:
  -h, --help              Show help
  --version               Print version
  -a, --auto              Auto-select fastest server (lowest latency)
  -S, --server string     Server alias or URL
  --servers               List configured servers
  -p, --protocol string   Protocol: tcp, udp, quic (default: tcp)
  -d, --direction string  Direction: download, upload, bidirectional (default: download)
  -t, --duration int      Test duration in seconds (1-300) (default: 30)
  -s, --streams int       Parallel streams (1-16) (default: 4)
  --packet-size int       Packet size in bytes (64-9000) (default: 1500)
  --json                  Output results as JSON
  --plain                 Plain text output
  -v, --verbose           Verbose output
  -q, --quiet             Quiet mode (errors only)
  --no-color              Disable color output
  --no-progress           Disable progress indicators
  --timeout int           Request timeout in seconds (default: 60)
  --api-key string        API key for authentication
  --server-url string     Override server URL

Measurement:
  --warmup int            Warm-up seconds before measurement (default: 2)

Configuration file: ~/.config/obyte/config.yaml

Environment variables:
  OBYTE_SERVER_URL        Server URL (default: http://localhost:8080)
  OBYTE_API_KEY           API key
  NO_COLOR                Disable colors

Examples:
  obyte                                    # Default test
  obyte -a                                 # Auto-select fastest server
  obyte -d upload -t 60                   # Upload test, 60s
  obyte -p udp -d bidirectional -s 8      # UDP bidirectional, 8 streams
  obyte --json server.example.com         # JSON output
`)
}
