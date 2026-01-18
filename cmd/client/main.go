package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"
)

const (
	version = "0.2.0"

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
	defaultPacketSize = 1500
	defaultTimeout    = 60
)

func main() {
	configFile, err := loadConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "obyte: warning: failed to load config file: %v\n", err)
	}

	flagConfig, flagsSet := parseFlags()

	config := mergeConfig(flagConfig, configFile, flagsSet)

	if config.Auto {
		fastest, err := selectFastestServer(configFile, config.Verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "obyte: error: %v\n", err)
			os.Exit(exitFailure)
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
		fmt.Fprintf(os.Stderr, "obyte: error: %v\n", err)
		os.Exit(exitUsage)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Timeout)*time.Second)
	defer cancel()

	if !config.JSON && !config.Plain {
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			config.Plain = true
		}
	}

	formatter := createFormatter(config)

	var streamID string

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		if streamID != "" {
			cancelStream(config.ServerURL, streamID, config.APIKey)
		}
		cancel()
		os.Exit(exitInterrupt)
	}()

	if err := runStream(ctx, config, formatter, &streamID); err != nil {
		formatter.FormatError(err)
		os.Exit(exitFailure)
	}
}
