package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
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

func TestRouterRejectsInvalidStreamID(t *testing.T) {
	router := &api.Router{}
	called := false

	handler := router.HandleWithID(func(w http.ResponseWriter, r *http.Request, streamID string) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/stream/bad/status", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "bad"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if called {
		t.Fatalf("handler should not be called for invalid stream id")
	}
}
