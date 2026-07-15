package api

import "net/http"

func (r *Router) newRateLimitedV1Registrar(mux *http.ServeMux) func(method, route string, handler http.HandlerFunc) {
	return func(method, route string, handler http.HandlerFunc) {
		mux.HandleFunc(method+" "+apiV1Prefix+route, applyRateLimit(r.limiter, handler))
	}
}

func (r *Router) registerResultsAPIRoutes(v1 func(method, route string, handler http.HandlerFunc)) {
	if r.resultsHandler == nil {
		return
	}
	v1("POST", "/results", r.resultsHandler.save)
	v1("GET", "/results/{id}", r.resultsHandler.get)
}

func (r *Router) registerResultsPageRoute(mux *http.ServeMux, webFS http.FileSystem) {
	if r.resultsHandler == nil {
		return
	}
	resultsPageHandler := func(w http.ResponseWriter, req *http.Request) {
		if !validResultID(req.PathValue("id")) {
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
	handler = r.CORSMiddleware(handler)
	handler = rejectBodylessRequestBodies(handler)
	handler = SecurityHeadersMiddleware(handler)
	handler = r.LoggingMiddleware(handler)
	return handler
}
