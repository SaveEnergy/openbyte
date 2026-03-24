package api

import (
	"net/http"
	"strings"

	"github.com/saveenergy/openbyte/pkg/types"
)

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
				respondJSON(w, map[string]string{"error": errOriginNotAllowed}, http.StatusForbidden)
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
		if matchesAllowedOrigin(strings.TrimSpace(allowed), origin, originHostValue) {
			return true
		}
	}
	return false
}

func matchesAllowedOrigin(allowed, origin, originHostValue string) bool {
	if allowed == "" {
		return false
	}
	if allowed == "*" || strings.EqualFold(allowed, origin) {
		return true
	}
	if after, ok := strings.CutPrefix(allowed, "*."); ok {
		return originHostValue != "" &&
			(originHostValue == after || strings.HasSuffix(originHostValue, "."+after))
	}
	allowedHost := types.OriginHost(allowed)
	return allowedHost != "" && originHostValue != "" && strings.EqualFold(allowedHost, originHostValue)
}

func (r *Router) isAllowAllOrigins() bool {
	return r.corsAllowAll
}
