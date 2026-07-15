package api

import (
	"net/http"
	"strings"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/internal/results"
	"github.com/saveenergy/openbyte/web"
)

type Router struct {
	version          string
	serverName       string
	speedtest        *SpeedTestHandler
	resultsHandler   *resultHandler
	limiter          *RateLimiter
	allowedOrigins   []string
	corsAllowAll     bool
	clientIPResolver *ClientIPResolver
	webFS            http.FileSystem
}

func NewRouter(cfg *config.Config, version string, resultsStore *results.Store) *Router {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	maxDur := 300
	if cfg.MaxTestDuration > 0 {
		maxDur = int(cfg.MaxTestDuration.Seconds())
	}
	resolver := NewClientIPResolver(cfg)
	speedtest := NewSpeedTestHandler(cfg.MaxConcurrentHTTP(), maxDur)
	speedtest.SetMaxConcurrentPerIP(cfg.MaxConcurrentPerIP)
	speedtest.SetClientIPResolver(resolver)

	if version == "" {
		version = "dev"
	}
	serverName := strings.TrimSpace(cfg.ServerName)
	if serverName == "" {
		serverName = config.DefaultServerName
	}
	webFS := http.FileSystem(http.FS(web.Assets))
	if cfg.WebRoot != "" {
		webFS = http.Dir(cfg.WebRoot)
	}
	allowedOrigins, corsAllowAll := normalizeAllowedOrigins(cfg.AllowedOrigins)
	return &Router{
		version:          version,
		serverName:       serverName,
		speedtest:        speedtest,
		resultsHandler:   newResultHandler(resultsStore),
		limiter:          newRateLimiter(cfg, resolver),
		allowedOrigins:   allowedOrigins,
		corsAllowAll:     corsAllowAll,
		clientIPResolver: resolver,
		webFS:            webFS,
	}
}

func (r *Router) SetupRoutes() http.Handler {
	mux := http.NewServeMux()
	webFS := r.resolveWebFS()
	rateLimitedV1 := r.newRateLimitedV1Registrar(mux)

	rateLimitedV1("GET", "/version", r.GetVersion)
	r.registerResultsAPIRoutes(rateLimitedV1)
	mux.HandleFunc("GET "+apiV1Prefix+"/download", r.speedtest.Download)
	mux.HandleFunc("POST "+apiV1Prefix+"/upload", r.speedtest.Upload)
	mux.HandleFunc("GET "+apiV1Prefix+"/ping", r.speedtest.Ping)

	mux.HandleFunc("GET /health", r.HealthCheck)
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, req *http.Request) {
		respondJSON(w, map[string]string{"error": errNotFound}, http.StatusNotFound)
	})

	r.registerResultsPageRoute(mux, webFS)

	staticHandler := staticCacheMiddleware(newStaticAllowlistHandler(webFS))
	mux.Handle("/", staticHandler)

	return r.wrapMiddlewares(mux)
}

func (r *Router) HealthCheck(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		logging.Warn("health: write response", logging.Field{Key: "error", Value: err})
	}
}

func normalizeAllowedOrigins(origins []string) ([]string, bool) {
	if len(origins) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(origins))
	allowAll := false
	for _, o := range origins {
		t := strings.TrimSpace(o)
		if t == "" {
			continue
		}
		out = append(out, t)
		if t == "*" {
			allowAll = true
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, allowAll
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
