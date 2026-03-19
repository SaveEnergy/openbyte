package server

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
)

var (
	exitSuccess = 0
	exitFailure = 1
)

func Run(args []string, version string) int {
	logLevel := logging.LevelInfo
	if os.Getenv("LOG_LEVEL") == config.EnvDebug {
		logLevel = logging.LevelDebug
	}
	logging.Init(logLevel)

	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		logging.Error("Failed to load config", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	fs, fv := buildServerFlagSet(cfg)
	versionFlag := fs.Bool("version", false, "Print version")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		logging.Error("Invalid flags", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	if *versionFlag {
		fmt.Printf("openbyte %s\n", version)
		return exitSuccess
	}
	if err := applyServerFlagOverrides(cfg, fs, fv); err != nil {
		logging.Error("Invalid flag values", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	if err := cfg.Validate(); err != nil {
		logging.Error("Invalid configuration", logging.Field{Key: "error", Value: err})
		return exitFailure
	}

	pprofServer := startPprofServer(cfg)
	stopStats := startRuntimeStatsLogger(cfg)
	cleanupOnError := true

	resources := &serverResources{}
	defer func() {
		if !cleanupOnError {
			return
		}
		resources.stopAll(pprofServer, stopStats)
	}()

	muxRouter, err := setupRuntimeResources(cfg, version, resources)
	if err != nil {
		return exitFailure
	}

	srv := &http.Server{
		Addr:              cfg.BindAddress + ":" + cfg.Port,
		Handler:           muxRouter,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		HTTP2: &http.HTTP2Config{
			StrictMaxConcurrentRequests: true,
		},
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	srvErrCh := make(chan error, 1)
	startHTTPServer(cfg, srv, srvErrCh)
	exitCode := waitForShutdown(quit, srvErrCh)
	shutdownHTTPServer(srv, 30*time.Second)

	resources.stopAll(pprofServer, stopStats)
	cleanupOnError = false

	logging.Info("Server stopped")
	return exitCode
}
