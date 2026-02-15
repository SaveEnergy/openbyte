package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/term"
)

var (
	exitSuccess   = 0
	exitFailure   = 1
	exitUsage     = 2
	exitInterrupt = 130
)

const (
	defaultServerURL  = "http://localhost:8080"
	defaultProtocol   = "tcp"
	defaultDirection  = "download"
	defaultDuration   = 30
	defaultStreams    = 4
	defaultPacketSize = 1400
	defaultChunkSize  = 1024 * 1024
	defaultTimeout    = 60
	defaultWarmUp     = 2
)

func Run(args []string, version string) int {
	configFile, err := loadConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "openbyte client: warning: failed to load config file: %v\n", err)
	}

	flagConfig, flagsSet, exitCode, err := parseFlags(args, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "openbyte client: error: %v\n", err)
		return exitUsage
	}
	if flagConfig == nil {
		return exitCode
	}

	config := mergeConfig(flagConfig, configFile, flagsSet)

	// Create formatter early so pre-test errors are structured when --json is set.
	if !config.JSON && !config.NDJSON && !config.Plain {
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			config.Plain = true
		}
	}
	formatter := createFormatter(config)

	if config.Auto {
		fastest, selectErr := selectFastestServer(configFile, config.Verbose)
		if selectErr != nil {
			formatter.FormatError(selectErr)
			return exitFailure
		}
		config.ServerURL = fastest.URL
		if !config.Quiet {
			name := fastest.Name
			if name == "" {
				name = fastest.Alias
			}
			fmt.Printf("Auto-selected: %s (%dms)\n\n", name, fastest.Latency.Milliseconds())
		}
	}

	if err := validateConfig(config); err != nil {
		formatter.FormatError(err)
		return exitUsage
	}

	// Timeout covers the entire lifecycle: ping/RTT + test duration + overhead.
	// Add test duration to the base timeout so the timeout doesn't cut the test short.
	totalTimeout := time.Duration(config.Timeout)*time.Second + time.Duration(config.Duration)*time.Second
	ctx, cancel := context.WithTimeout(context.Background(), totalTimeout)
	defer cancel()

	var streamID atomic.Value
	var interrupted atomic.Bool

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		interrupted.Store(true)
		if id, ok := streamID.Load().(string); ok && id != "" {
			if err := CancelStream(ctx, config.ServerURL, id, config.APIKey); err != nil {
				fmt.Fprintf(os.Stderr, "openbyte client: warning: failed to cancel stream %s: %v\n", id, err)
			}
		}
		cancel()
	}()

	if err := runStream(ctx, config, formatter, &streamID); err != nil {
		if interrupted.Load() && errors.Is(err, context.Canceled) {
			return exitInterrupt
		}
		formatter.FormatError(err)
		return exitFailure
	}
	if interrupted.Load() {
		return exitInterrupt
	}
	return exitSuccess
}
