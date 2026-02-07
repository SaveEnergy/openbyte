package api

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/internal/results"
	"github.com/saveenergy/openbyte/pkg/types"
	"github.com/saveenergy/openbyte/web"
)

type Router struct {
	handler          *Handler
	speedtest        *SpeedTestHandler
	resultsHandler   *results.Handler
	limiter          *RateLimiter
	wsServer         interface{}
	allowedOrigins   []string
	clientIPResolver *ClientIPResolver
	webFS            http.FileSystem
}

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
// If path is empty, the embedded assets are used.
func (r *Router) SetWebRoot(path string) {
	if path != "" {
		r.webFS = http.Dir(path)
	}
}

func (r *Router) SetupRoutes() *mux.Router {
	router := mux.NewRouter()
	router.Use(r.LoggingMiddleware)
	router.Use(SecurityHeadersMiddleware)
	router.Use(r.CORSMiddleware)

	v1 := router.PathPrefix("/api/v1").Subrouter()

	if r.limiter != nil {
		v1.Use(RateLimitMiddleware(r.limiter))
	}

	v1.HandleFunc("/stream/start", r.handler.StartStream).Methods("POST")
	v1.HandleFunc("/stream/{id}/status", r.HandleWithID(r.handler.GetStreamStatus)).Methods("GET")
	v1.HandleFunc("/stream/{id}/results", r.HandleWithID(r.handler.GetStreamResults)).Methods("GET")
	v1.HandleFunc("/stream/{id}/cancel", r.HandleWithID(r.handler.CancelStream)).Methods("POST")
	v1.HandleFunc("/stream/{id}/metrics", r.HandleWithID(r.handler.ReportMetrics)).Methods("POST")
	v1.HandleFunc("/stream/{id}/complete", r.HandleWithID(r.handler.CompleteStream)).Methods("POST")
	v1.HandleFunc("/servers", r.handler.GetServers).Methods("GET")
	v1.HandleFunc("/version", r.handler.GetVersion).Methods("GET")

	// Speedtest routes (use same subrouter but skip rate limiting internally via handler)
	v1.HandleFunc("/download", r.speedtest.Download).Methods("GET")
	v1.HandleFunc("/upload", r.speedtest.Upload).Methods("POST")
	v1.HandleFunc("/ping", r.speedtest.Ping).Methods("GET")

	// Saved results API
	if r.resultsHandler != nil {
		v1.HandleFunc("/results", r.resultsHandler.Save).Methods("POST")
		v1.HandleFunc("/results/{id}", r.resultsHandler.Get).Methods("GET")
	}

	if r.wsServer != nil {
		if wsHandler, ok := r.wsServer.(func(http.ResponseWriter, *http.Request, string)); ok {
			v1.HandleFunc("/stream/{id}/stream", func(w http.ResponseWriter, req *http.Request) {
				vars := mux.Vars(req)
				streamID := vars["id"]
				if streamID == "" {
					respondJSON(w, map[string]string{"error": "stream ID required"}, http.StatusBadRequest)
					return
				}
				wsHandler(w, req, streamID)
			}).Methods("GET")
		}
	}

	router.HandleFunc("/health", r.HealthCheck).Methods("GET")

	webFS := r.resolveWebFS()

	// Serve results.html for /results/{id} browser requests
	if r.resultsHandler != nil {
		router.HandleFunc("/results/{id}", func(w http.ResponseWriter, req *http.Request) {
			f, err := webFS.Open("results.html")
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
			http.ServeContent(w, req, "results.html", stat.ModTime(), f)
		}).Methods("GET")
	}

	router.PathPrefix("/").Handler(http.FileServer(webFS))

	return router
}

func (r *Router) HandleWithID(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		streamID := vars["id"]
		if streamID == "" {
			respondJSON(w, map[string]string{"error": "stream ID required"}, http.StatusBadRequest)
			return
		}
		if !isValidStreamID(streamID) {
			respondJSON(w, map[string]string{"error": "invalid stream ID"}, http.StatusBadRequest)
			return
		}
		fn(w, req, streamID)
	}
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

func (r *Router) CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		origin := req.Header.Get("Origin")
		originAllowed := origin != "" && r.isAllowedOrigin(origin)
		if originAllowed {
			allowOrigin := origin
			if r.isAllowAllOrigins() {
				allowOrigin = "*"
			}
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")
			if allowOrigin != "*" {
				w.Header().Add("Vary", "Origin")
			}
		}
		if req.Method == http.MethodOptions {
			if origin != "" && !originAllowed {
				respondJSON(w, map[string]string{"error": "origin not allowed"}, http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func (r *Router) isAllowedOrigin(origin string) bool {
	if len(r.allowedOrigins) == 0 {
		return false
	}
	originHostValue := types.OriginHost(origin)
	for _, allowed := range r.allowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if allowed == "*" {
			return true
		}
		if strings.EqualFold(allowed, origin) {
			return true
		}
		if strings.HasPrefix(allowed, "*.") {
			suffix := strings.TrimPrefix(allowed, "*.")
			if originHostValue != "" && (originHostValue == suffix || strings.HasSuffix(originHostValue, "."+suffix)) {
				return true
			}
		}
		allowedHost := types.OriginHost(allowed)
		if allowedHost != "" && originHostValue != "" && strings.EqualFold(allowedHost, originHostValue) {
			return true
		}
	}
	return false
}

func (r *Router) isAllowAllOrigins() bool {
	for _, allowed := range r.allowedOrigins {
		if allowed == "*" {
			return true
		}
	}
	return false
}

func isValidStreamID(streamID string) bool {
	if streamID == "" {
		return false
	}
	_, err := uuid.Parse(streamID)
	return err == nil
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not implement http.Hijacker")
}

func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *Router) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path

		skipLog := strings.HasSuffix(path, "/stream") ||
			strings.HasSuffix(path, "/download") ||
			strings.HasSuffix(path, "/upload") ||
			strings.HasSuffix(path, "/ping")

		if strings.HasPrefix(path, "/api/") && !skipLog {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, req)

			duration := time.Since(start)
			logging.Info("HTTP request",
				logging.Field{Key: "method", Value: req.Method},
				logging.Field{Key: "path", Value: path},
				logging.Field{Key: "status", Value: rw.statusCode},
				logging.Field{Key: "duration_ms", Value: float64(duration.Microseconds()) / 1000},
				logging.Field{Key: "ip", Value: r.resolveClientIP(req)},
			)
		} else {
			next.ServeHTTP(w, req)
		}
	})
}

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"font-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"script-src 'self'; "+
				"img-src 'self' data:; "+
				"connect-src *")
		next.ServeHTTP(w, r)
	})
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
