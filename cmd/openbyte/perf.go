package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // Registers pprof handlers on DefaultServeMux.
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func startPprofServer(cfg *config.Config) *http.Server {
	if cfg == nil || !cfg.PprofEnabled {
		return nil
	}

	srv := &http.Server{
		Addr:    cfg.PprofAddress,
		Handler: http.DefaultServeMux,
	}

	go func() {
		slog.Info("pprof server starting", "address", cfg.PprofAddress)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("pprof server failed", "error", err)
		}
	}()

	return srv
}

func shutdownPprofServer(srv *http.Server, timeout time.Duration) {
	if srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Warn("pprof server shutdown error", "error", err)
	}
}
