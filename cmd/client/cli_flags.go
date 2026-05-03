package client

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const (
	flagChunkSize  = "chunk-size"
	flagServerURL  = "server-url"
	helpHintSuffix = "See: openbyte client --help"
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
	flagSet.IntVar(&config.ChunkSize, flagChunkSize, 0, "HTTP chunk size in bytes (65536-4194304)")
	flagSet.BoolVar(&config.JSON, "json", false, "Output results as JSON")
	flagSet.BoolVar(&config.NDJSON, "ndjson", false, "Streaming newline-delimited JSON output")
	flagSet.BoolVar(&config.Plain, "plain", false, "Plain text output")
	flagSet.BoolVar(&config.Verbose, "verbose", false, "Verbose output")
	flagSet.BoolVar(&config.Verbose, "v", false, "Verbose output (short)")
	flagSet.BoolVar(&config.Quiet, "quiet", false, "Quiet mode (errors only)")
	flagSet.BoolVar(&config.Quiet, "q", false, "Quiet mode (errors only) (short)")
	flagSet.BoolVar(&config.NoColor, "no-color", false, "Disable color output")
	flagSet.BoolVar(&config.NoProgress, "no-progress", false, "Disable progress indicators")
	flagSet.StringVar(&config.ServerURL, "S", "", "Server URL (short)")
	flagSet.StringVar(&config.ServerURL, flagServerURL, "", "Server URL (override)")
	flagSet.IntVar(&config.Timeout, "timeout", 0, "Request timeout in seconds")

	flagSet.IntVar(&config.WarmUp, "warmup", 2, "Warm-up seconds before measurement")

	versionFlag := flagSet.Bool("version", false, "Print version")
	help := flagSet.Bool("help", false, "Show help")
	flagSet.BoolVar(help, "h", false, "Show help (short)")

	if err := flagSet.Parse(args); err != nil {
		return nil, nil, exitUsage, err
	}

	flagSet.Visit(func(f *flag.Flag) {
		flagsSet[f.Name] = true
		applyFlagAlias(flagsSet, f.Name)
	})

	if *versionFlag {
		fmt.Printf("openbyte %s\n", version)
		return nil, nil, exitSuccess, nil
	}

	if *help {
		printUsage()
		return nil, nil, exitSuccess, nil
	}

	rest := flagSet.Args()
	if err := applyPositionalServerArg(config, flagsSet, rest); err != nil {
		return nil, nil, exitUsage, err
	}

	return config, flagsSet, 0, nil
}

func applyFlagAlias(flagsSet map[string]bool, name string) {
	switch name {
	case "p":
		flagsSet["protocol"] = true
	case "d":
		flagsSet["direction"] = true
	case "t":
		flagsSet["duration"] = true
	case "s":
		flagsSet["streams"] = true
	case flagChunkSize:
		flagsSet[flagChunkSize] = true
	case "S":
		flagsSet[flagServerURL] = true
	case "v":
		flagsSet["verbose"] = true
	case "q":
		flagsSet["quiet"] = true
	case "h":
		flagsSet["help"] = true
	}
}

func applyPositionalServerArg(config *Config, flagsSet map[string]bool, rest []string) error {
	if len(rest) > 1 {
		return fmt.Errorf("too many positional arguments")
	}
	if len(rest) == 0 {
		return nil
	}
	server := rest[0]
	if strings.HasPrefix(server, "http://") || strings.HasPrefix(server, "https://") {
		normalized, err := normalizeAndValidateServerURL(server)
		if err != nil {
			return fmt.Errorf("invalid server URL: %w", err)
		}
		config.ServerURL = normalized
		flagsSet[flagServerURL] = true
		return nil
	}
	if strings.TrimSpace(server) == "" {
		return fmt.Errorf("server URL is required")
	}
	return fmt.Errorf("invalid server URL %q: include http:// or https://", server)
}
