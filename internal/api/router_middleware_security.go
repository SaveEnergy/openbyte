package api

import (
	"net/http"
	"time"
)

func isHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return forwardedProtoIsHTTPS(r.Header.Get("X-Forwarded-Proto"))
}

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"font-src 'self'; "+
				"style-src 'self'; "+
				"script-src 'self'; "+
				"worker-src 'self'; "+
				"img-src 'self' data:; "+
				"connect-src 'self' https: http:")
		if isHTTPS(r) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func DeadlineMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deadline, ok := r.Context().Deadline(); ok {
			controller := http.NewResponseController(w)
			_ = controller.SetWriteDeadline(deadline.Add(5 * time.Second))
		}
		next.ServeHTTP(w, r)
	})
}
