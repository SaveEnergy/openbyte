package api

import "net/http"

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
