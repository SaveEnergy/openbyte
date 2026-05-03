package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/stream"
)

const versionEndpoint = "/api/v1/version"

func TestGetVersion(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)
	handler.SetVersion("1.2.3")

	req := httptest.NewRequest(http.MethodGet, versionEndpoint, nil)
	rec := httptest.NewRecorder()
	handler.GetVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}

	var resp api.VersionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode version response: %v", err)
	}
	if resp.Version != "1.2.3" {
		t.Errorf("version = %q, want 1.2.3", resp.Version)
	}
}

func TestGetVersionDefault(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, versionEndpoint, nil)
	rec := httptest.NewRecorder()
	handler.GetVersion(rec, req)

	var resp api.VersionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode default version response: %v", err)
	}
	if resp.Version != "dev" {
		t.Errorf("default version = %q, want dev", resp.Version)
	}
}

func TestGetVersionDrainsUnexpectedBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)
	tb := &trackingBody{data: []byte(`{"unexpected":"payload"}`)}

	req := httptest.NewRequest(http.MethodGet, versionEndpoint, nil)
	req.Body = tb
	rec := httptest.NewRecorder()
	handler.GetVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}
	assertTrackingBodyDrained(t, tb)
}
