package results_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/results"
)

func TestSaveReturnsInternalErrorWhenStoreFails(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "results.db")

	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	// Force save path to fail by closing DB before handler call.
	store.Close()

	h := results.NewHandler(store)

	body := `{
		"download_mbps": 100,
		"upload_mbps": 50,
		"latency_ms": 10,
		"jitter_ms": 1,
		"loaded_latency_ms": 12,
		"bufferbloat_grade": "A",
		"ipv4": "203.0.113.10",
		"ipv6": "",
		"server_name": "test"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/results", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Save(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["error"]; got != "failed to save result" {
		t.Fatalf("error = %q, want %q", got, "failed to save result")
	}

	_ = os.Remove(dbPath)
}

func TestGetReturnsNoStoreForSavedResult(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "results.db")

	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	id, err := store.Save(results.Result{
		DownloadMbps:     100,
		UploadMbps:       50,
		LatencyMs:        10,
		JitterMs:         1,
		LoadedLatencyMs:  12,
		BufferbloatGrade: "A",
		IPv4:             "203.0.113.10",
		ServerName:       "test",
	})
	if err != nil {
		t.Fatalf("save result: %v", err)
	}

	h := results.NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/results/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want %q", got, "no-store")
	}
}

func TestGetReturnsNotFoundForMissingResult(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "results.db")

	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	h := results.NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/results/abc12345", nil)
	req.SetPathValue("id", "abc12345")
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetRejectsInvalidResultID(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "results.db")

	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	h := results.NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/results/bad-id", nil)
	req.SetPathValue("id", "bad-id")
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSaveSucceedsReturns201WithIDAndURL(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "results.db")

	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	h := results.NewHandler(store)
	body := `{
		"download_mbps": 100,
		"upload_mbps": 50,
		"latency_ms": 10,
		"jitter_ms": 1,
		"loaded_latency_ms": 12,
		"bufferbloat_grade": "A",
		"ipv4": "203.0.113.10",
		"ipv6": "",
		"server_name": "test"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/results", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Save(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	id := resp["id"]
	url := resp["url"]
	if len(id) != 8 {
		t.Fatalf("id = %q, want 8-char id", id)
	}
	if url != "/results/"+id {
		t.Fatalf("url = %q, want %q", url, "/results/"+id)
	}
}

func TestGetReturnsInternalErrorWhenStoreFails(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "results.db")

	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	store.Close()

	h := results.NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/results/abc12345", nil)
	req.SetPathValue("id", "abc12345")
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["error"]; got != "internal error" {
		t.Fatalf("error = %q, want %q", got, "internal error")
	}
}
