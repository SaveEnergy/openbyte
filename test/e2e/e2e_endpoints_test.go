package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.baseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var data map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	if data["status"] != "ok" {
		t.Errorf("Health status = %s, want 'ok'", data["status"])
	}
}

func TestPingEndpoint(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.baseURL + "/api/v1/ping")
	if err != nil {
		t.Fatalf("Ping request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Ping status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Cache-Control"); got != noStoreValue {
		t.Fatalf("Cache-Control = %q, want %q", got, noStoreValue)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode ping response: %v", err)
	}
	if pong, ok := data["pong"].(bool); !ok || !pong {
		t.Fatalf("pong = %v, want true", data["pong"])
	}
	if _, ok := data["timestamp"].(float64); !ok {
		t.Fatalf("timestamp missing or invalid type: %T", data["timestamp"])
	}
}

func TestResultsSaveAndGet(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	saveReq := map[string]any{
		"download_mbps":     123.45,
		"upload_mbps":       67.89,
		"latency_ms":        12.3,
		"jitter_ms":         1.2,
		"loaded_latency_ms": 18.4,
		"bufferbloat_grade": "A",
		"ipv4":              "192.0.2.1",
		"ipv6":              "",
		"server_name":       "e2e-server",
	}
	body, err := json.Marshal(saveReq)
	if err != nil {
		t.Fatalf("marshal save request: %v", err)
	}

	resp, err := http.Post(ts.baseURL+"/api/v1/results", jsonContentType, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("save result request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("save result status = %d, want %d, body=%s", resp.StatusCode, http.StatusCreated, string(data))
	}

	var saveResp struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&saveResp); err != nil {
		t.Fatalf("decode save response: %v", err)
	}
	if saveResp.ID == "" || saveResp.URL == "" {
		t.Fatalf("save response missing id/url: %#v", saveResp)
	}

	getResp, err := http.Get(ts.baseURL + "/api/v1/results/" + saveResp.ID)
	if err != nil {
		t.Fatalf("get result request failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(getResp.Body)
		t.Fatalf("get result status = %d, want %d, body=%s", getResp.StatusCode, http.StatusOK, string(data))
	}
	if got := getResp.Header.Get("Cache-Control"); got != noStoreValue {
		t.Fatalf("get result cache-control = %q, want %q", got, noStoreValue)
	}

	var saved map[string]any
	if err := json.NewDecoder(getResp.Body).Decode(&saved); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if id, ok := saved["id"].(string); !ok || id != saveResp.ID {
		t.Fatalf("saved id = %#v, want %q", saved["id"], saveResp.ID)
	}
}
