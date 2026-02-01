package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/internal/quic"
	"github.com/saveenergy/openbyte/internal/registry"
	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/internal/websocket"
	"github.com/saveenergy/openbyte/pkg/types"
)

var version = "dev"

func main() {
	logLevel := logging.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = logging.LevelDebug
	}
	logging.Init(logLevel)

	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		logging.Error("Failed to load config", logging.Field{Key: "error", Value: err})
		log.Fatalf("Failed to load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		logging.Error("Invalid configuration", logging.Field{Key: "error", Value: err})
		log.Fatalf("Invalid configuration: %v", err)
	}

	pprofServer := startPprofServer(cfg)
	startRuntimeStatsLogger(cfg)

	streamServer, err := stream.NewServer(cfg)
	if err != nil {
		logging.Error("Failed to start stream server", logging.Field{Key: "error", Value: err})
		log.Fatalf("Failed to start stream server: %v", err)
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

	router := api.NewRouter(apiHandler)
	router.SetRateLimiter(cfg)
	router.SetClientIPResolver(api.NewClientIPResolver(cfg))
	router.SetAllowedOrigins(cfg.AllowedOrigins)
	router.SetWebSocketHandler(wsServer.HandleStream)
	router.SetWebRoot(cfg.WebRoot)
	muxRouter := router.SetupRoutes()

	var registryService *registry.Service
	if cfg.RegistryMode {
		logging.Info("Starting in registry mode")
		registryService = registry.NewService(cfg.RegistryServerTTL, 30*time.Second)
		registryService.Start()

		registryLogger := logging.NewLogger("registry")
		registryHandler := registry.NewHandler(registryService, registryLogger, cfg.RegistryAPIKey)
		registryHandler.RegisterRoutes(muxRouter)
	}

	go broadcastMetrics(manager, wsServer)

	var registryClient *registry.Client
	if cfg.RegistryEnabled && !cfg.RegistryMode {
		logger := logging.NewLogger("registry-client")
		registryClient = registry.NewClient(cfg, logger)
		if err := registryClient.Start(manager.ActiveCount); err != nil {
			logging.Warn("Registry client failed to start", logging.Field{Key: "error", Value: err})
		}
	}

	// Start QUIC server if enabled
	var quicServer *quic.Server
	if cfg.QUICEnabled {
		tlsConfig, err := quic.GetTLSConfig(cfg)
		if err != nil {
			logging.Error("Failed to get TLS config for QUIC", logging.Field{Key: "error", Value: err})
			log.Fatalf("Failed to get TLS config: %v", err)
		}

		quicServer, err = quic.NewServer(cfg, tlsConfig)
		if err != nil {
			logging.Error("Failed to create QUIC server", logging.Field{Key: "error", Value: err})
			log.Fatalf("Failed to create QUIC server: %v", err)
		}

		if err := quicServer.Start(); err != nil {
			logging.Error("Failed to start QUIC server", logging.Field{Key: "error", Value: err})
			log.Fatalf("Failed to start QUIC server: %v", err)
		}
	}

	srv := &http.Server{
		Addr:         cfg.BindAddress + ":" + cfg.Port,
		Handler:      muxRouter,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		fields := []logging.Field{
			{Key: "address", Value: cfg.BindAddress + ":" + cfg.Port},
			{Key: "tcp_test", Value: cfg.GetTCPTestAddress()},
			{Key: "udp_test", Value: cfg.GetUDPTestAddress()},
		}
		if cfg.QUICEnabled {
			fields = append(fields, logging.Field{Key: "quic", Value: cfg.GetQUICAddress()})
		}
		logging.Info("Server starting", fields...)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Error("Server failed", logging.Field{Key: "error", Value: err})
			log.Fatalf("Server failed: %v", err)
		}
	}()

	sig := <-quit
	logging.Info("Shutting down server...", logging.Field{Key: "signal", Value: sig.String()})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logging.Error("Server shutdown error", logging.Field{Key: "error", Value: err})
		log.Printf("Server shutdown error: %v", err)
	}

	shutdownPprofServer(pprofServer, 5*time.Second)

	if registryClient != nil {
		registryClient.Stop()
	}
	if registryService != nil {
		registryService.Stop()
	}

	if quicServer != nil {
		quicServer.Close()
	}

	wsServer.Close()
	manager.Stop()
	streamServer.Close()

	logging.Info("Server stopped")
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
