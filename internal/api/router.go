package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/internal/results"
	"github.com/saveenergy/openbyte/web"
)

type Router struct {
	handler          *Handler
	speedtest        *SpeedTestHandler
	resultsHandler   *results.Handler
	limiter          *RateLimiter
	allowedOrigins   []string
	corsAllowAll     bool // true when allowedOrigins contains "*"; set in SetAllowedOrigins
	clientIPResolver *ClientIPResolver
	webFS            http.FileSystem
	runtimeMetrics   http.HandlerFunc
}

var validResultID = regexp.MustCompile(`^[0-9a-zA-Z]{8}$`)

func NewRouter(handler *Handler, cfg *config.Config) *Router {
	maxDur := 300
	if cfg.MaxTestDuration > 0 {
		maxDur = int(cfg.MaxTestDuration.Seconds())
	}
	speedtest := NewSpeedTestHandler(cfg.MaxConcurrentHTTP(), maxDur)
	speedtest.SetMaxConcurrentPerIP(cfg.MaxConcurrentPerIP)
	return &Router{
		handler:   handler,
		speedtest: speedtest,
	}
}

func (r *Router) SetRateLimiter(cfg *config.Config) {
	r.limiter = NewRateLimiter(cfg)
}

func (r *Router) SetClientIPResolver(resolver *ClientIPResolver) {
	r.clientIPResolver = resolver
	if r.speedtest != nil {
		r.speedtest.SetClientIPResolver(resolver)
	}
}

func (r *Router) SetResultsHandler(h *results.Handler) {
	r.resultsHandler = h
}

// SetWebRoot overrides the embedded web assets with a directory on disk.
// Use this for development so you can edit HTML/CSS/JS without rebuilding.
// If webRootDir is empty, the embedded assets are used.
func (r *Router) SetWebRoot(webRootDir string) {
	if webRootDir != "" {
		r.webFS = http.Dir(webRootDir)
	}
}

func (r *Router) SetRuntimeMetricsHandler(handler http.HandlerFunc) {
	r.runtimeMetrics = handler
}

func (r *Router) SetupRoutes() http.Handler {
	mux := http.NewServeMux()
	webFS := r.resolveWebFS()
	v1 := r.newV1Registrar(mux)

	r.registerCoreV1Routes(v1)
	r.registerResultsAPIRoutes(v1)

	mux.HandleFunc("GET /health", r.HealthCheck)
	if r.runtimeMetrics != nil {
		mux.HandleFunc("GET /debug/runtime-metrics", r.runtimeMetrics)
	}
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, req *http.Request) {
		respondJSON(w, map[string]string{"error": errNotFound}, http.StatusNotFound)
	})

	r.registerResultsPageRoute(mux, webFS)

	staticHandler := staticCacheMiddleware(newStaticAllowlistHandler(webFS))
	mux.Handle("/", gzipMiddleware(staticHandler))

	return r.wrapMiddlewares(mux)
}

func (r *Router) HealthCheck(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		logging.Warn("health: write response", logging.Field{Key: "error", Value: err})
	}
}

func (r *Router) SetAllowedOrigins(origins []string) {
	r.corsAllowAll = false
	if len(origins) == 0 {
		r.allowedOrigins = nil
		return
	}
	out := make([]string, 0, len(origins))
	for _, o := range origins {
		t := strings.TrimSpace(o)
		if t == "" {
			continue
		}
		out = append(out, t)
		if t == "*" {
			r.corsAllowAll = true
		}
	}
	if len(out) == 0 {
		r.allowedOrigins = nil
		return
	}
	r.allowedOrigins = out
}

// resolveWebFS returns the web file system to use for static assets.
// If a disk override is set (via SetWebRoot), it takes precedence.
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
