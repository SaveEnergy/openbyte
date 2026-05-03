package api

import (
	"net/http"

	"github.com/google/uuid"
)

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
		if !isValidStreamID(streamID) {
			respondJSON(w, map[string]string{"error": errInvalidStreamID}, http.StatusBadRequest)
			return
		}
		if r.handler == nil || r.handler.manager == nil {
			respondJSON(w, map[string]string{"error": errNotFound}, http.StatusNotFound)
			return
		}
		if _, err := r.handler.manager.GetStream(streamID); err != nil {
			respondJSON(w, map[string]string{"error": errNotFound}, http.StatusNotFound)
			return
		}
		req = req.Clone(req.Context())
		req.Header = req.Header.Clone()
		req.Header.Set(internalWSClientIPHeader, r.resolveClientIP(req))
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
