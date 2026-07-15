package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/results"
	"github.com/saveenergy/openbyte/web"
)

type Router struct {
	serverName       string
	speedtest        *SpeedTestHandler
	resultsHandler   *resultHandler
	limiter          *RateLimiter
	clientIPResolver *ClientIPResolver
	webFS            http.FileSystem
}

func NewRouter(cfg *config.Config, resultsStore *results.Store) *Router {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	// Valid configuration is whole seconds. Invalid direct callers get the
	// safest usable limit instead of widening a truncated value to 300 seconds.
	maxDur := max(1, int(cfg.MaxTestDuration/time.Second))
	resolver := NewClientIPResolver(cfg)
	speedtest := NewSpeedTestHandler(cfg.MaxConcurrentHTTP(), maxDur, cfg.MaxConcurrentPerIP, resolver)

	serverName := strings.TrimSpace(cfg.ServerName)
	if serverName == "" {
		serverName = config.DefaultServerName
	}
	webFS := http.FileSystem(http.FS(web.Assets))
	if cfg.WebRoot != "" {
		webFS = http.Dir(cfg.WebRoot)
	}
	return &Router{
		serverName:       serverName,
		speedtest:        speedtest,
		resultsHandler:   newResultHandler(resultsStore),
		limiter:          newRateLimiter(cfg, resolver),
		clientIPResolver: resolver,
		webFS:            webFS,
	}
}

func (r *Router) SetupRoutes() http.Handler {
	mux := http.NewServeMux()
	webFS := r.resolveWebFS()
	rateLimitedV1 := r.newRateLimitedV1Registrar(mux)

	r.registerResultsAPIRoutes(rateLimitedV1)
	mux.HandleFunc("GET "+apiV1Prefix+"/download", r.speedtest.Download)
	mux.HandleFunc("POST "+apiV1Prefix+"/upload", r.speedtest.Upload)
	mux.HandleFunc("GET "+apiV1Prefix+"/ping", r.ping)

	mux.HandleFunc("GET /health", r.HealthCheck)
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, req *http.Request) {
		respondJSON(w, map[string]string{"error": errNotFound}, http.StatusNotFound)
	})

	staticHandler := staticCacheMiddleware(newStaticAllowlistHandler(webFS))
	r.registerResultsPageRoute(mux, staticHandler)
	mux.Handle("/", staticHandler)

	return r.wrapMiddlewares(mux)
}

func (r *Router) ping(w http.ResponseWriter, req *http.Request) {
	serverName := ""
	if req.URL.RawQuery != "" && req.URL.Query().Get("meta") == "1" {
		serverName = r.serverName
	}
	r.speedtest.ping(w, req, serverName)
}

func (r *Router) HealthCheck(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		slog.Warn("health: write response", "error", err)
	}
}

// resolveWebFS returns the web file system to use for static assets.
// If a disk override is configured, it takes precedence.
// Otherwise, the embedded assets from the web package are used.
func (r *Router) resolveWebFS() http.FileSystem {
	if r.webFS != nil {
		return r.webFS
	}
	return http.FS(web.Assets)
}

func (r *Router) resolveClientIP(req *http.Request) string {
	if r.clientIPResolver == nil {
		return ipString(parseRemoteIP(req.RemoteAddr))
	}
	return r.clientIPResolver.FromRequest(req)
}
