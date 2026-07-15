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
	if _, exists := response["server_name"]; exists {
		t.Fatalf("plain ping unexpectedly included server_name: %s", rec.Body.String())
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
