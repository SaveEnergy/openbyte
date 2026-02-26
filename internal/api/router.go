package api

import (
	"net/http"
	"regexp"

	"github.com/google/uuid"
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
	wsServer         any
	allowedOrigins   []string
	clientIPResolver *ClientIPResolver
	webFS            http.FileSystem
	runtimeMetrics   http.HandlerFunc
}

var validResultID = regexp.MustCompile(`^[0-9a-zA-Z]{8}$`)

func (r *Router) GetLimiter() *RateLimiter {
	return r.limiter
}

func NewRouter(handler *Handler, cfg *config.Config) *Router {
	maxDur := 300
	if cfg.MaxTestDuration > 0 {
		maxDur = int(cfg.MaxTestDuration.Seconds())
	}
	return &Router{
		handler:   handler,
		speedtest: NewSpeedTestHandler(cfg.MaxConcurrentHTTP(), maxDur),
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

func (r *Router) SetWebSocketHandler(handler func(http.ResponseWriter, *http.Request, string)) {
	r.wsServer = handler
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

// RouteRegistrar allows external packages to register routes on the
// ServeMux before middleware wrapping, without importing gorilla/mux.
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

func (r *Router) SetupRoutes(registrars ...RouteRegistrar) http.Handler {
	mux := http.NewServeMux()
	webFS := r.resolveWebFS()
	v1 := r.newV1Registrar(mux)

	r.registerCoreV1Routes(v1)
	r.registerResultsAPIRoutes(v1)
	r.registerWebSocketStreamRoute(v1)

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

	// Let external registrars add routes before middleware wrapping
	for _, reg := range registrars {
		reg.RegisterRoutes(mux)
	}

	return r.wrapMiddlewares(mux)
}

func (r *Router) newV1Registrar(mux *http.ServeMux) func(method, route string, handler http.HandlerFunc) {
	return func(method, route string, handler http.HandlerFunc) {
		h := handler
		if r.limiter != nil {
			h = applyRateLimit(r.limiter, h)
		}
		mux.HandleFunc(method+" "+apiV1Prefix+route, h)
	}
}

func (r *Router) registerCoreV1Routes(v1 func(method, route string, handler http.HandlerFunc)) {
	v1("POST", "/stream/start", r.handler.StartStream)
	v1("GET", "/stream/{id}/status", r.HandleWithID(r.handler.GetStreamStatus))
	v1("GET", "/stream/{id}/results", r.HandleWithID(r.handler.GetStreamResults))
	v1("POST", "/stream/{id}/cancel", r.HandleWithID(r.handler.CancelStream))
	v1("POST", "/stream/{id}/metrics", r.HandleWithID(r.handler.ReportMetrics))
	v1("POST", "/stream/{id}/complete", r.HandleWithID(r.handler.CompleteStream))
	v1("GET", "/servers", r.handler.GetServers)
	v1("GET", "/version", r.handler.GetVersion)
	v1("GET", "/download", r.speedtest.Download)
	v1("POST", "/upload", r.speedtest.Upload)
	v1("GET", "/ping", r.speedtest.Ping)
}

func (r *Router) registerResultsAPIRoutes(v1 func(method, route string, handler http.HandlerFunc)) {
	if r.resultsHandler == nil {
		return
	}
	v1("POST", "/results", r.resultsHandler.Save)
	v1("GET", "/results/{id}", r.resultsHandler.Get)
}

func (r *Router) registerWebSocketStreamRoute(v1 func(method, route string, handler http.HandlerFunc)) {
	if r.wsServer == nil {
		return
	}
	wsHandler, ok := r.wsServer.(func(http.ResponseWriter, *http.Request, string))
	if !ok {
		return
	}
	v1("GET", "/stream/{id}/stream", func(w http.ResponseWriter, req *http.Request) {
		streamID := req.PathValue("id")
		if streamID == "" {
			respondJSON(w, map[string]string{"error": errStreamIDRequired}, http.StatusBadRequest)
			return
		}
		wsHandler(w, req, streamID)
	})
}

func (r *Router) registerResultsPageRoute(mux *http.ServeMux, webFS http.FileSystem) {
	if r.resultsHandler == nil {
		return
	}
	resultsPageHandler := func(w http.ResponseWriter, req *http.Request) {
		if !validResultID.MatchString(req.PathValue("id")) {
			http.NotFound(w, req)
			return
		}
		w.Header().Set(headerCacheControl, valueNoStore)
		f, err := webFS.Open(resultsHTML)
		if err != nil {
			http.NotFound(w, req)
			return
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil {
			http.NotFound(w, req)
			return
		}
		http.ServeContent(w, req, resultsHTML, stat.ModTime(), f)
	}
	if r.limiter != nil {
		resultsPageHandler = applyRateLimit(r.limiter, resultsPageHandler)
	}
	mux.HandleFunc("GET /results/{id}", resultsPageHandler)
}

func (r *Router) wrapMiddlewares(handler http.Handler) http.Handler {
	if r.limiter != nil {
		handler = registryRateLimitMiddleware(r.limiter, handler)
	}
	handler = DeadlineMiddleware(handler)
	handler = r.CORSMiddleware(handler)
	handler = SecurityHeadersMiddleware(handler)
	handler = r.LoggingMiddleware(handler)
	return handler
}

func (r *Router) HandleWithID(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		streamID := req.PathValue("id")
		if streamID == "" {
			respondJSON(w, map[string]string{"error": errStreamIDRequired}, http.StatusBadRequest)
			return
		}
		if !isValidStreamID(streamID) {
			respondJSON(w, map[string]string{"error": errInvalidStreamID}, http.StatusBadRequest)
			return
		}
		fn(w, req, streamID)
	}
}

func isValidStreamID(streamID string) bool {
	if streamID == "" {
		return false
	}
	_, err := uuid.Parse(streamID)
	return err == nil
}

func (r *Router) HealthCheck(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		logging.Warn("health: write response", logging.Field{Key: "error", Value: err})
	}
}

func (r *Router) SetAllowedOrigins(origins []string) {
	r.allowedOrigins = origins
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
