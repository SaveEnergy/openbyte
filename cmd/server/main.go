package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/internal/registry"
	"github.com/saveenergy/openbyte/internal/results"
	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/internal/websocket"
	"github.com/saveenergy/openbyte/pkg/types"
)

var (
	exitSuccess = 0
	exitFailure = 1
)

func Run(version string) int {
	logLevel := logging.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = logging.LevelDebug
	}
	logging.Init(logLevel)

	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		logging.Error("Failed to load config", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	if err := cfg.Validate(); err != nil {
		logging.Error("Invalid configuration", logging.Field{Key: "error", Value: err})
		return exitFailure
	}

	pprofServer := startPprofServer(cfg)
	stopStats := startRuntimeStatsLogger(cfg)

	streamServer, err := stream.NewServer(cfg)
	if err != nil {
		logging.Error("Failed to start stream server", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	logging.Info("Stream server started",
		logging.Field{Key: "tcp_port", Value: cfg.TCPTestPort},
		logging.Field{Key: "udp_port", Value: cfg.UDPTestPort})

	manager := stream.NewManager(cfg.MaxConcurrentTests, cfg.MaxConcurrentPerIP)
	manager.SetRetentionPeriod(cfg.TestRetentionPeriod)
	manager.SetMetricsUpdateInterval(cfg.MetricsUpdateInterval)
	manager.Start()

	apiHandler := api.NewHandler(manager)
	apiHandler.SetConfig(cfg)
	apiHandler.SetVersion(version)
	wsServer := websocket.NewServer()
	wsServer.SetAllowedOrigins(cfg.AllowedOrigins)
	wsServer.SetPingInterval(cfg.WebSocketPingInterval)

	// Ensure data directory exists for SQLite
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		logging.Error("Failed to create data directory", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	resultsStore, err := results.New(cfg.DataDir+"/results.db", cfg.MaxStoredResults)
	if err != nil {
		logging.Error("Failed to open results store", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	logging.Info("Results store opened",
		logging.Field{Key: "path", Value: cfg.DataDir + "/results.db"},
		logging.Field{Key: "max_results", Value: cfg.MaxStoredResults})

	router := api.NewRouter(apiHandler, cfg)
	router.SetRateLimiter(cfg)
	router.SetClientIPResolver(api.NewClientIPResolver(cfg))
	router.SetAllowedOrigins(cfg.AllowedOrigins)
	router.SetWebSocketHandler(wsServer.HandleStream)
	router.SetResultsHandler(results.NewHandler(resultsStore))
	router.SetWebRoot(cfg.WebRoot)

	var registryService *registry.Service
	var registrars []api.RegistryRegistrar
	if cfg.RegistryMode {
		logging.Info("Starting in registry mode")
		registryService = registry.NewService(cfg.RegistryServerTTL, 30*time.Second)
		registryService.Start()

		registryLogger := logging.NewLogger("registry")
		registryHandler := registry.NewHandler(registryService, registryLogger, cfg.RegistryAPIKey)
		registrars = append(registrars, registryHandler)
	}

	muxRouter := router.SetupRoutes(registrars...)

	var broadcastWg sync.WaitGroup
	broadcastWg.Add(1)
	go func() {
		defer broadcastWg.Done()
		broadcastMetrics(manager, wsServer)
	}()

	var registryClient *registry.Client
	if cfg.RegistryEnabled && !cfg.RegistryMode {
		logger := logging.NewLogger("registry-client")
		registryClient = registry.NewClient(cfg, logger)
		if err := registryClient.Start(manager.ActiveCount); err != nil {
			logging.Warn("Registry client failed to start", logging.Field{Key: "error", Value: err})
		}
	}

	srv := &http.Server{
		Addr:              cfg.BindAddress + ":" + cfg.Port,
		Handler:           muxRouter,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	srvErrCh := make(chan error, 1)

	go func() {
		fields := []logging.Field{
			{Key: "address", Value: cfg.BindAddress + ":" + cfg.Port},
			{Key: "tcp_test", Value: cfg.GetTCPTestAddress()},
			{Key: "udp_test", Value: cfg.GetUDPTestAddress()},
		}
		logging.Info("Server starting", fields...)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErrCh <- err
		}
	}()

	exitCode := exitSuccess
	select {
	case sig := <-quit:
		logging.Info("Shutting down server...", logging.Field{Key: "signal", Value: sig.String()})
	case err := <-srvErrCh:
		logging.Error("Server failed", logging.Field{Key: "error", Value: err})
		exitCode = exitFailure
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logging.Error("Server shutdown error", logging.Field{Key: "error", Value: err})
	}

	shutdownPprofServer(pprofServer, 5*time.Second)
	stopStats()

	if registryClient != nil {
		registryClient.Stop()
	}
	if registryService != nil {
		registryService.Stop()
	}

	resultsStore.Close()
	manager.Stop()
	broadcastWg.Wait()
	wsServer.Close()
	streamServer.Close()

	logging.Info("Server stopped")
	return exitCode
}

func broadcastMetrics(manager *stream.Manager, wsServer *websocket.Server) {
	updateCh := manager.GetMetricsUpdateChannel()

	for update := range updateCh {
		if update.State.Status == types.StreamStatusRunning ||
			update.State.Status == types.StreamStatusStarting ||
			update.State.Status == types.StreamStatusCompleted ||
			update.State.Status == types.StreamStatusFailed {
			wsServer.BroadcastMetrics(update.StreamID, update.State)
		}
	}
}
