package server

import (
	"context"
	"errors"
	"net/http"
	_ "net/http/pprof" // Registers pprof handlers on DefaultServeMux.
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
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
		logging.Info("pprof server starting", logging.Field{Key: "address", Value: cfg.PprofAddress})
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.Error("pprof server failed", logging.Field{Key: "error", Value: err})
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
		logging.Warn("pprof server shutdown error", logging.Field{Key: "error", Value: err})
	}
}
