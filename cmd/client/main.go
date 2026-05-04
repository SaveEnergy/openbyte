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
	defaultServerURL = "http://localhost:8080"
	defaultDirection = "download"
	defaultDuration  = 30
	defaultStreams   = 4
	defaultChunkSize = 1024 * 1024
	defaultTimeout   = 60
	defaultWarmUp    = 2
)

// Protocol/direction literals for validation and branching (S1192).
const (
	protocolHTTP      = "http"
	directionDownload = "download"
	directionUpload   = "upload"
	statusCompleted   = "completed"
	schemeHTTP        = "http"
	schemeHTTPS       = "https"
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

	ensureTTYFormatterDefaults(config)
	formatter := createFormatter(config)

	if err := validateConfig(config); err != nil {
		formatter.FormatError(err)
		return exitUsage
	}

	// Timeout covers the entire lifecycle: ping/RTT + test duration + overhead.
	// Add test duration to the base timeout so the timeout doesn't cut the test short.
	totalTimeout := time.Duration(config.Timeout)*time.Second + time.Duration(config.Duration)*time.Second
	ctx, cancel := context.WithTimeout(context.Background(), totalTimeout)
	defer cancel()

	var interrupted atomic.Bool

	stopInterruptWatcher := startInterruptWatcher(cancel, &interrupted)
	defer stopInterruptWatcher()

	return executeClientRun(ctx, config, formatter, &interrupted)
}

func startInterruptWatcher(
	cancel context.CancelFunc,
	interrupted *atomic.Bool,
) func() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
			return
		case <-sigCh:
			interrupted.Store(true)
			cancel()
		}
	}()
	return func() {
		close(done)
		signal.Stop(sigCh)
	}
}

func executeClientRun(
	ctx context.Context,
	config *Config,
	formatter OutputFormatter,
	interrupted *atomic.Bool,
) int {
	if err := runStream(ctx, config, formatter); err != nil {
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

func ensureTTYFormatterDefaults(config *Config) {
	if !config.JSON && !config.NDJSON && !config.Plain && !term.IsTerminal(int(os.Stdout.Fd())) {
		config.Plain = true
	}
}
