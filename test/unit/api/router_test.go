package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
)

func TestRouterAllowedOriginWildcard(t *testing.T) {
	router := &api.Router{}
	router.SetAllowedOrigins([]string{"*.example.com"})

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("Origin", "https://foo.example.com")
	rec := httptest.NewRecorder()

	handler := router.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://foo.example.com" {
		t.Fatalf("allow origin = %q, want %q", got, "https://foo.example.com")
	}
}

func TestRouterAllowedOriginHostMatch(t *testing.T) {
	router := &api.Router{}
	router.SetAllowedOrigins([]string{"foo.example.com"})

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("Origin", "https://foo.example.com:8443")
	rec := httptest.NewRecorder()

	handler := router.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://foo.example.com:8443" {
		t.Fatalf("allow origin = %q, want %q", got, "https://foo.example.com:8443")
	}
}
