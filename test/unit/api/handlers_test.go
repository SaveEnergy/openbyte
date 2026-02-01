package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/stream"
)

func TestStartStreamRejectsLargeBody(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	payload := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  10,
		"streams":   1,
		"padding":   strings.Repeat("a", (1<<20)+256),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.StartStream(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}
