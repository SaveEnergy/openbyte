package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
)

func TestVersionEndpoint(t *testing.T) {
	handler := api.NewHandler(nil)
	handler.SetVersion("1.2.3")

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/version", nil)
	rec := httptest.NewRecorder()

	handler.GetVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Version != "1.2.3" {
		t.Fatalf("version = %q, want %q", resp.Version, "1.2.3")
	}
}
