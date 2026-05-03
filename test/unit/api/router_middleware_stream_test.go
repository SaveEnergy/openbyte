package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
)

func TestRouterAllowedOriginWildcard(t *testing.T) {
	router := &api.Router{}
	router.SetAllowedOrigins([]string{"*.example.com"})

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL, nil)
	req.Header.Set("Origin", fooOrigin)
	rec := httptest.NewRecorder()

	handler := router.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(allowedOriginKey); got != fooOrigin {
		t.Fatalf(routerAllowOriginFmt, got, fooOrigin)
	}
}

func TestRouterAllowedOriginHostMatch(t *testing.T) {
	router := &api.Router{}
	router.SetAllowedOrigins([]string{"foo.example.com"})

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL, nil)
	req.Header.Set("Origin", fooOriginWithPort)
	rec := httptest.NewRecorder()

	handler := router.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(allowedOriginKey); got != fooOriginWithPort {
		t.Fatalf(routerAllowOriginFmt, got, fooOriginWithPort)
	}
}

func TestRouterAllowedOriginSetAllowedOriginsTrimsWhitespace(t *testing.T) {
	router := &api.Router{}
	router.SetAllowedOrigins([]string{"  https://foo.example.com  "})

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL, nil)
	req.Header.Set("Origin", fooOrigin)
	rec := httptest.NewRecorder()

	handler := router.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(allowedOriginKey); got != fooOrigin {
		t.Fatalf(routerAllowOriginFmt, got, fooOrigin)
	}
}

func TestRouterRejectsWildcardBypassOrigin(t *testing.T) {
	router := &api.Router{}
	router.SetAllowedOrigins([]string{"*.example.com"})

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL, nil)
	req.Header.Set("Origin", evilOrigin)
	rec := httptest.NewRecorder()

	handler := router.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(allowedOriginKey); got != "" {
		t.Fatalf(routerEvilBypassFmt, got)
	}
}

func TestRouterRejectsInvalidStreamID(t *testing.T) {
	router := &api.Router{}
	called := false

	handler := router.HandleWithID(func(w http.ResponseWriter, r *http.Request, streamID string) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/api/v1/stream/bad/status", nil)
	req.SetPathValue("id", "bad")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusBadRequest)
	}
	if called {
		t.Fatal(routerInvalidIDCalledErr)
	}
}

func TestWebSocketRouteRejectsInvalidStreamID(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())
	called := false
	router.SetWebSocketHandler(func(w http.ResponseWriter, r *http.Request, streamID string) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	h := router.SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/api/v1/stream/bad/stream", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusBadRequest)
	}
	if called {
		t.Fatal(routerInvalidIDCalledErr)
	}
}

func TestWebSocketRouteRejectsUnknownStreamBeforeUpgrade(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())
	called := false
	router.SetWebSocketHandler(func(w http.ResponseWriter, r *http.Request, streamID string) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	h := router.SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+streamWSAPIPath, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusNotFound)
	}
	if called {
		t.Fatal("websocket handler should not be called for unknown stream")
	}
}
