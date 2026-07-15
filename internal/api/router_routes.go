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

func (r *Router) registerResultsPageRoute(mux *http.ServeMux, staticHandler http.Handler) {
	if r.resultsHandler == nil {
		return
	}
	resultsPageHandler := func(w http.ResponseWriter, req *http.Request) {
		if !validResultID(req.PathValue("id")) {
			http.NotFound(w, req)
			return
		}
		staticReq := req.Clone(req.Context())
		staticReq.URL.Path = "/" + resultsHTML
		staticReq.URL.RawPath = ""
		staticHandler.ServeHTTP(w, staticReq)
	}
	if r.limiter != nil {
		resultsPageHandler = applyRateLimit(r.limiter, resultsPageHandler)
	}
	mux.HandleFunc("GET /results/{id}", resultsPageHandler)
}

func (r *Router) wrapMiddlewares(handler http.Handler) http.Handler {
	handler = rejectBodylessRequestBodies(handler)
	handler = SecurityHeadersMiddleware(handler)
	handler = r.LoggingMiddleware(handler)
	return handler
}
