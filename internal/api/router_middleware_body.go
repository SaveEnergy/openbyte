package api

import (
	"net/http"

	"github.com/saveenergy/openbyte/internal/httpbody"
)

func rejectBodylessRequestBodies(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			httpbody.Abort(w, r)
		}
		next.ServeHTTP(w, r)
	})
}
