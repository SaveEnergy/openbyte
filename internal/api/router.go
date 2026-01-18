package api

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
)

type Router struct {
	handler        *Handler
	speedtest      *SpeedTestHandler
	limiter        *RateLimiter
	wsServer       interface{}
	allowedOrigins []string
	clientIPResolver *ClientIPResolver
	webRoot        string
}

func (r *Router) GetLimiter() *RateLimiter {
	return r.limiter
}

func NewRouter(handler *Handler) *Router {
	return &Router{
		handler:   handler,
		speedtest: NewSpeedTestHandler(20),
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

func (r *Router) SetWebRoot(path string) {
	r.webRoot = path
}

func (r *Router) SetupRoutes() *mux.Router {
	router := mux.NewRouter()
	router.Use(r.LoggingMiddleware)
	router.Use(r.CORSMiddleware)

	v1 := router.PathPrefix("/api/v1").Subrouter()

	if r.limiter != nil {
		v1.Use(RateLimitMiddleware(r.limiter))
	}

	v1.Methods("OPTIONS").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	v1.HandleFunc("/stream/start", r.handler.StartStream).Methods("POST")
	v1.HandleFunc("/stream/{id}/status", r.HandleWithID(r.handler.GetStreamStatus)).Methods("GET")
	v1.HandleFunc("/stream/{id}/results", r.HandleWithID(r.handler.GetStreamResults)).Methods("GET")
	v1.HandleFunc("/stream/{id}/cancel", r.HandleWithID(r.handler.CancelStream)).Methods("POST")
	v1.HandleFunc("/stream/{id}/metrics", r.HandleWithID(r.handler.ReportMetrics)).Methods("POST")
	v1.HandleFunc("/stream/{id}/complete", r.HandleWithID(r.handler.CompleteStream)).Methods("POST")
	v1.HandleFunc("/servers", r.handler.GetServers).Methods("GET")

	// Speedtest routes (use same subrouter but skip rate limiting internally via handler)
	v1.HandleFunc("/download", r.speedtest.Download).Methods("GET")
	v1.HandleFunc("/upload", r.speedtest.Upload).Methods("POST")
	v1.HandleFunc("/ping", r.speedtest.Ping).Methods("GET")

	if r.wsServer != nil {
		if wsHandler, ok := r.wsServer.(func(http.ResponseWriter, *http.Request, string)); ok {
			v1.HandleFunc("/stream/{id}/stream", func(w http.ResponseWriter, req *http.Request) {
				vars := mux.Vars(req)
				streamID := vars["id"]
				if streamID == "" {
					http.Error(w, "stream ID required", http.StatusBadRequest)
					return
				}
				wsHandler(w, req, streamID)
			}).Methods("GET")
		}
	}

	router.HandleFunc("/health", r.HealthCheck).Methods("GET")
	webRoot := r.webRoot
	if webRoot == "" {
		webRoot = "./web"
	}
	router.PathPrefix("/").Handler(http.FileServer(http.Dir(webRoot)))

	return router
}

func (r *Router) HandleWithID(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		streamID := vars["id"]
		if streamID == "" {
			http.Error(w, "stream ID required", http.StatusBadRequest)
			return
		}
		fn(w, req, streamID)
	}
}

func (r *Router) HealthCheck(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
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
				http.Error(w, "Origin not allowed", http.StatusForbidden)
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
	originHostValue := originHost(origin)
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
			if originHostValue != "" && strings.HasSuffix(originHostValue, suffix) {
				return true
			}
		}
		allowedHost := originHost(allowed)
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

func originHost(origin string) string {
	parsed, err := url.Parse(origin)
	if err == nil && parsed.Host != "" {
		return stripPort(parsed.Host)
	}
	return stripPort(origin)
}

func stripPort(host string) string {
	if host == "" {
		return host
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	}
	return host
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

func (r *Router) resolveClientIP(req *http.Request) string {
	if r.clientIPResolver == nil {
		return ipString(parseRemoteIP(req.RemoteAddr))
	}
	return r.clientIPResolver.FromRequest(req)
}
