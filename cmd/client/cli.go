package client

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func parseFlags(args []string, version string) (*Config, map[string]bool, int, error) {
	config := &Config{}
	flagsSet := make(map[string]bool)

	flagSet := flag.NewFlagSet("openbyte client", flag.ContinueOnError)
	flagSet.SetOutput(os.Stdout)
	flagSet.StringVar(&config.Protocol, "protocol", "", "Protocol: tcp, udp, http")
	flagSet.StringVar(&config.Protocol, "p", "", "Protocol: tcp, udp, http (short)")
	flagSet.StringVar(&config.Direction, "direction", "", "Direction: download, upload, bidirectional")
	flagSet.StringVar(&config.Direction, "d", "", "Direction: download, upload, bidirectional (short)")
	flagSet.IntVar(&config.Duration, "duration", 0, "Test duration in seconds (1-300)")
	flagSet.IntVar(&config.Duration, "t", 0, "Test duration in seconds (1-300) (short)")
	flagSet.IntVar(&config.Streams, "streams", 0, "Parallel streams (1-64)")
	flagSet.IntVar(&config.Streams, "s", 0, "Parallel streams (1-64) (short)")
	flagSet.IntVar(&config.PacketSize, "packet-size", 0, "Packet size in bytes (64-9000)")
	flagSet.IntVar(&config.ChunkSize, "chunk-size", 0, "HTTP chunk size in bytes (65536-4194304)")
	flagSet.BoolVar(&config.JSON, "json", false, "Output results as JSON")
	flagSet.BoolVar(&config.NDJSON, "ndjson", false, "Streaming newline-delimited JSON output")
	flagSet.BoolVar(&config.Plain, "plain", false, "Plain text output")
	flagSet.BoolVar(&config.Verbose, "verbose", false, "Verbose output")
	flagSet.BoolVar(&config.Verbose, "v", false, "Verbose output (short)")
	flagSet.BoolVar(&config.Quiet, "quiet", false, "Quiet mode (errors only)")
	flagSet.BoolVar(&config.Quiet, "q", false, "Quiet mode (errors only) (short)")
	flagSet.BoolVar(&config.NoColor, "no-color", false, "Disable color output")
	flagSet.BoolVar(&config.NoProgress, "no-progress", false, "Disable progress indicators")
	flagSet.StringVar(&config.Server, "server", "", "Server alias or URL")
	flagSet.StringVar(&config.Server, "S", "", "Server alias or URL (short)")
	flagSet.StringVar(&config.ServerURL, "server-url", "", "Server URL (override)")
	flagSet.StringVar(&config.APIKey, "api-key", "", "API key for authentication")
	flagSet.IntVar(&config.Timeout, "timeout", 0, "Request timeout in seconds")

	flagSet.IntVar(&config.WarmUp, "warmup", 2, "Warm-up seconds before measurement")
	flagSet.BoolVar(&config.Auto, "auto", false, "Auto-select fastest server")
	flagSet.BoolVar(&config.Auto, "a", false, "Auto-select fastest server (short)")

	versionFlag := flagSet.Bool("version", false, "Print version")
	help := flagSet.Bool("help", false, "Show help")
	flagSet.BoolVar(help, "h", false, "Show help (short)")
	servers := flagSet.Bool("servers", false, "List configured servers")

	if err := flagSet.Parse(args); err != nil {
		return nil, nil, exitUsage, err
	}

	flagSet.Visit(func(f *flag.Flag) {
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
		case "chunk-size":
			flagsSet["chunk-size"] = true
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
		return nil, nil, exitSuccess, nil
	}

	if *versionFlag {
		fmt.Printf("openbyte %s\n", version)
		return nil, nil, exitSuccess, nil
	}

	if *help {
		printUsage()
		return nil, nil, exitSuccess, nil
	}

	rest := flagSet.Args()
	if len(rest) > 0 {
		server := rest[0]
		if strings.HasPrefix(server, "http://") || strings.HasPrefix(server, "https://") {
			config.ServerURL = server
			flagsSet["server-url"] = true
		} else {
			config.Server = server
			flagsSet["server"] = true
		}
	}

	return config, flagsSet, 0, nil
}

func listServers() {
	configFile, err := loadConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "openbyte client: warning: %v\n", err)
	}

	fmt.Println("Configured Servers:")
	fmt.Println()

	if configFile == nil || len(configFile.Servers) == 0 {
		fmt.Println("  No servers configured.")
		fmt.Println()
		fmt.Println("Add servers to ~/.config/openbyte/config.yaml:")
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
	aliases := make([]string, 0, len(configFile.Servers))
	for alias := range configFile.Servers {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		server := configFile.Servers[alias]
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
	fmt.Println("Usage: openbyte client -S <alias> or openbyte client <alias>")
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
			if resp != nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}

			if err != nil {
				result.Error = err
			} else {
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
	if config.Protocol != "tcp" && config.Protocol != "udp" && config.Protocol != "http" {
		return fmt.Errorf("invalid protocol: %s\n\n"+
			"Protocol must be 'tcp', 'udp', or 'http'.\n"+
			"Use: openbyte client -p tcp  or  openbyte client -p udp  or  openbyte client -p http\n"+
			"See: openbyte client --help", config.Protocol)
	}
	if config.Protocol == "http" && config.Direction == "bidirectional" {
		return fmt.Errorf("invalid direction for http: %s\n\n"+
			"HTTP protocol supports 'download' or 'upload'.\n"+
			"Use: openbyte client -p http -d download  or  openbyte client -p http -d upload\n"+
			"See: openbyte client --help", config.Direction)
	}
	if config.Direction != "download" && config.Direction != "upload" && config.Direction != "bidirectional" {
		return fmt.Errorf("invalid direction: %s\n\n"+
			"Direction must be 'download', 'upload', or 'bidirectional'.\n"+
			"Use: openbyte client -d download  or  openbyte client -d upload  or  openbyte client -d bidirectional\n"+
			"See: openbyte client --help", config.Direction)
	}
	if config.Duration < 1 || config.Duration > 300 {
		return fmt.Errorf("invalid duration: %d\n\n"+
			"Duration must be between 1 and 300 seconds.\n"+
			"Use: openbyte client -t 30  (for 30 seconds)\n"+
			"See: openbyte client --help", config.Duration)
	}
	if config.Streams < 1 || config.Streams > 64 {
		return fmt.Errorf("invalid streams: %d\n\n"+
			"Streams must be between 1 and 64.\n"+
			"Use: openbyte client -s 4  (for 4 parallel streams)\n"+
			"See: openbyte client --help", config.Streams)
	}
	if config.Protocol != "http" {
		if config.PacketSize < 64 || config.PacketSize > 9000 {
			return fmt.Errorf("invalid packet size: %d\n\n"+
				"Packet size must be between 64 and 9000 bytes.\n"+
				"Use: openbyte client --packet-size 1400  (WAN-safe default)\n"+
				"See: openbyte client --help", config.PacketSize)
		}
	}
	if config.Protocol == "http" {
		if config.ChunkSize < 65536 || config.ChunkSize > 4194304 {
			return fmt.Errorf("invalid chunk size: %d\n\n"+
				"Chunk size must be between 65536 and 4194304 bytes.\n"+
				"Use: openbyte client --chunk-size 1048576  (1MB)\n"+
				"See: openbyte client --help", config.ChunkSize)
		}
	}
	return nil
}

func printUsage() {
	fmt.Fprintf(os.Stdout, `Usage: openbyte client [flags] [server]

Run network speed test (client-side measurement).

Server Selection:
  openbyte client <alias>           Use server alias from config
  openbyte client <url>             Use server URL directly
  openbyte client -S <alias>        Select server by alias
  openbyte client -a, --auto        Auto-select fastest server
  openbyte client --servers         List configured servers

Flags:
  -h, --help              Show help
  --version               Print version
  -a, --auto              Auto-select fastest server (lowest latency)
  -S, --server string     Server alias or URL
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
  --api-key string        API key for authentication
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
