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

	ensureTTYFormatterDefaults(config)
	formatter := createFormatter(config)

	if err := applyAutoServerSelection(config, configFile, formatter); err != nil {
		return err.code
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

	stopInterruptWatcher := startInterruptWatcher(ctx, cancel, config, &streamID, &interrupted)
	defer stopInterruptWatcher()

	return executeStreamRun(ctx, config, formatter, &streamID, &interrupted)
}

func startInterruptWatcher(
	ctx context.Context,
	cancel context.CancelFunc,
	config *Config,
	streamID *atomic.Value,
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
			if id, ok := streamID.Load().(string); ok && id != "" {
				if err := CancelStream(ctx, config.ServerURL, id, config.APIKey); err != nil {
					fmt.Fprintf(os.Stderr, "openbyte client: warning: failed to cancel stream %s: %v\n", id, err)
				}
			}
			cancel()
		}
	}()
	return func() {
		close(done)
		signal.Stop(sigCh)
	}
}

func executeStreamRun(
	ctx context.Context,
	config *Config,
	formatter OutputFormatter,
	streamID *atomic.Value,
	interrupted *atomic.Bool,
) int {
	if err := runStream(ctx, config, formatter, streamID); err != nil {
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

type runFailure struct {
	code int
}

func applyAutoServerSelection(config *Config, configFile *ConfigFile, formatter OutputFormatter) *runFailure {
	if !config.Auto {
		return nil
	}
	fastest, selectErr := selectFastestServer(configFile, config.Verbose)
	if selectErr != nil {
		formatter.FormatError(selectErr)
		return &runFailure{code: exitFailure}
	}
	config.ServerURL = fastest.URL
	if !config.Quiet {
		name := fastest.Name
		if name == "" {
			name = fastest.Alias
		}
		fmt.Printf("Auto-selected: %s (%dms)\n\n", name, fastest.Latency.Milliseconds())
	}
	return nil
}
