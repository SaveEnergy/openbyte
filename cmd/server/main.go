package server

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

var (
	exitSuccess = 0
	exitFailure = 1
)

func Run(args []string, version string) int {
	versionFlag, err := parseServerArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		slog.Error("Invalid flags", "error", err)
		return exitFailure
	}
	if versionFlag {
		fmt.Printf("openbyte %s\n", version)
		return exitSuccess
	}
	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		slog.Error("Failed to load config", "error", err)
		return exitFailure
	}
	if err := cfg.Validate(); err != nil {
		slog.Error("Invalid configuration", "error", err)
		return exitFailure
	}

	muxRouter, resultsStore, err := setupRuntimeResources(cfg)
	if err != nil {
		return exitFailure
	}
	pprofServer := startPprofServer(cfg)

	srv := &http.Server{
		Addr:              cfg.BindAddress + ":" + cfg.Port,
		Handler:           muxRouter,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		HTTP2:             speedtestHTTP2Config(cfg),
	}
	configureHTTPProtocols(cfg, srv)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	srvErrCh := make(chan error, 1)
	startHTTPServer(cfg, srv, srvErrCh)
	exitCode := waitForShutdown(quit, srvErrCh)
	shutdownHTTPServer(srv, 30*time.Second)

	resultsStore.Close()
	shutdownPprofServer(pprofServer, 5*time.Second)
	slog.Info("Server stopped")
	return exitCode
}

func parseServerArgs(args []string) (bool, error) {
	fs := flag.NewFlagSet("openbyte", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "Usage: openbyte [--version]")
		fmt.Fprintln(os.Stdout, "\nServer configuration is environment-only; see README.md for variables.")
	}
	version := fs.Bool("version", false, "Print version")
	if err := fs.Parse(args); err != nil {
		return false, err
	}
	if fs.NArg() != 0 {
		return false, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	return *version, nil
}

func speedtestHTTP2Config(cfg *config.Config) *http.HTTP2Config {
	maxStreams := 100
	if cfg != nil {
		maxStreams = max(maxStreams, cfg.MaxConcurrentTransfers)
	}
	const receiveWindow = 4*1024*1024 - 1
	return &http.HTTP2Config{
		MaxConcurrentStreams:          maxStreams,
		MaxReadFrameSize:              1024 * 1024,
		MaxReceiveBufferPerConnection: receiveWindow,
		MaxReceiveBufferPerStream:     receiveWindow,
	}
}

func configureHTTPProtocols(cfg *config.Config, srv *http.Server) {
	if cfg == nil || cfg.HTTP2Enabled {
		return
	}
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	srv.Protocols = protocols
}
