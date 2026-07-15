package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
)

func TestPingBootstrapIncludesServerName(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ServerName = "Frankfurt 10G"
	handler := api.NewRouter(cfg, nil).SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, pingAPIPath+"?meta=1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusOK)
	}
	var response map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode ping metadata: %v", err)
	}
	if len(response) != 2 {
		t.Fatalf("metadata ping fields = %v, want client_ip and server_name", response)
	}
	if got := response["client_ip"]; got != "192.0.2.1" {
		t.Fatalf("client_ip = %v, want 192.0.2.1", got)
	}
	if got := response["server_name"]; got != "Frankfurt 10G" {
		t.Fatalf("server_name = %v, want Frankfurt 10G", got)
	}
}

func TestPlainPingOmitsServerName(t *testing.T) {
	handler := api.NewRouter(config.DefaultConfig(), nil).SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, pingAPIPath, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var response map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode ping: %v", err)
	}
	if len(response) != 1 {
		t.Fatalf("plain ping fields = %v, want only client_ip", response)
	}
	if _, exists := response["client_ip"]; !exists {
		t.Fatalf("plain ping missing client_ip: %s", rec.Body.String())
	}
}

func TestCrossOriginAccessIsLimitedToPing(t *testing.T) {
	handler := api.NewRouter(config.DefaultConfig(), nil).SetupRoutes()
	tests := []struct {
		path       string
		wantStatus int
		wantOrigin string
	}{
		{path: pingAPIPath, wantStatus: http.StatusOK, wantOrigin: "*"},
		{path: healthRoutePath, wantStatus: http.StatusOK},
		{path: apiUnknownPath, wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != tt.wantStatus {
			t.Errorf("%s: status = %d, want %d", tt.path, rec.Code, tt.wantStatus)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantOrigin {
			t.Errorf("%s: Access-Control-Allow-Origin = %q, want %q", tt.path, got, tt.wantOrigin)
		}
	}
}

func TestVersionEndpointIsRemoved(t *testing.T) {
	handler := api.NewRouter(config.DefaultConfig(), nil).SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, versionAPIPath, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusNotFound)
	}
}
