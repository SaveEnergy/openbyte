package results_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"encoding/json"

	"github.com/gorilla/mux"
	"github.com/saveenergy/openbyte/internal/results"
)

func tempStore(t *testing.T, maxResults int) (*results.Store, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := results.New(dbPath, maxResults)
	if err != nil {
		t.Fatalf("New store: %v", err)
	}
	return s, func() { s.Close(); os.RemoveAll(dir) }
}

func TestStoreSaveAndGet(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	r := results.Result{
		DownloadMbps:     123.4,
		UploadMbps:       56.7,
		LatencyMs:        12.3,
		JitterMs:         1.5,
		LoadedLatencyMs:  25.0,
		BufferbloatGrade: "A",
		IPv4:             "1.2.3.4",
		IPv6:             "::1",
		ServerName:       "Test Server",
	}

	id, err := store.Save(r)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if len(id) != 8 {
		t.Fatalf("expected 8-char ID, got %q", id)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.DownloadMbps != 123.4 {
		t.Errorf("download_mbps = %v, want 123.4", got.DownloadMbps)
	}
	if got.UploadMbps != 56.7 {
		t.Errorf("upload_mbps = %v, want 56.7", got.UploadMbps)
	}
	if got.IPv4 != "1.2.3.4" {
		t.Errorf("ipv4 = %q, want 1.2.3.4", got.IPv4)
	}
	if got.BufferbloatGrade != "A" {
		t.Errorf("bufferbloat_grade = %q, want A", got.BufferbloatGrade)
	}
}

func TestStoreGetNotFound(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	got, err := store.Get("abcd1234")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing ID, got %+v", got)
	}
}

func TestStoreTrimToMax(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "trim.db")

	// Create store with max 3 results
	store, err := results.New(dbPath, 3)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ids := make([]string, 5)
	for i := 0; i < 5; i++ {
		id, saveErr := store.Save(results.Result{
			DownloadMbps: float64(i + 1),
			UploadMbps:   1,
			LatencyMs:    1,
			JitterMs:     0,
		})
		if saveErr != nil {
			t.Fatalf("Save %d: %v", i, saveErr)
		}
		ids[i] = id
	}
	store.Close()

	// Reopen — cleanup runs on startup, should trim to 3
	store2, err := results.New(dbPath, 3)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer store2.Close()

	// Oldest 2 should be gone
	for _, id := range ids[:2] {
		got, getErr := store2.Get(id)
		if getErr != nil {
			t.Fatalf("Get %s: %v", id, getErr)
		}
		if got != nil {
			t.Errorf("expected id %s to be trimmed, but found it", id)
		}
	}
	// Newest 3 should remain
	for _, id := range ids[2:] {
		got, getErr := store2.Get(id)
		if getErr != nil {
			t.Fatalf("Get %s: %v", id, getErr)
		}
		if got == nil {
			t.Errorf("expected id %s to remain, but not found", id)
		}
	}
}

func TestHandlerSaveValidation(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	handler := results.NewHandler(store)
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/results", handler.Save).Methods("POST")
	router.HandleFunc("/api/v1/results/{id}", handler.Get).Methods("GET")

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"valid", `{"download_mbps":100,"upload_mbps":50,"latency_ms":10,"jitter_ms":1}`, http.StatusCreated},
		{"negative download", `{"download_mbps":-1,"upload_mbps":50,"latency_ms":10,"jitter_ms":1}`, http.StatusBadRequest},
		{"out of range", `{"download_mbps":200000,"upload_mbps":50,"latency_ms":10,"jitter_ms":1}`, http.StatusBadRequest},
		{"invalid json", `{bad}`, http.StatusBadRequest},
		{"empty body", ``, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/results", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d; body: %s", rec.Code, tt.status, rec.Body.String())
			}
		})
	}
}

func TestHandlerGetInvalidID(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	handler := results.NewHandler(store)
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/results/{id}", handler.Get).Methods("GET")

	tests := []struct {
		name   string
		id     string
		status int
	}{
		{"too short", "abc", http.StatusBadRequest},
		{"too long", "abcdefgh1234", http.StatusBadRequest},
		{"special chars", "abc-_...", http.StatusBadRequest},
		{"valid but missing", "abcd1234", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/results/"+tt.id, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
		})
	}
}

func TestHandlerRoundTrip(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	handler := results.NewHandler(store)
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/results", handler.Save).Methods("POST")
	router.HandleFunc("/api/v1/results/{id}", handler.Get).Methods("GET")

	// Save
	body := `{"download_mbps":500.5,"upload_mbps":100.2,"latency_ms":8.1,"jitter_ms":0.5,"loaded_latency_ms":15.3,"bufferbloat_grade":"B","ipv4":"203.0.113.1","ipv6":"2001:db8::1","server_name":"Test"}`
	req := httptest.NewRequest("POST", "/api/v1/results", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("save status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	var saveResp struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&saveResp); err != nil {
		t.Fatalf("decode save response: %v", err)
	}
	if saveResp.ID == "" {
		t.Fatal("empty ID in response")
	}
	if saveResp.URL != "/results/"+saveResp.ID {
		t.Errorf("url = %q, want /results/%s", saveResp.URL, saveResp.ID)
	}

	// Fetch
	req = httptest.NewRequest("GET", "/api/v1/results/"+saveResp.ID, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var result results.Result
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if result.DownloadMbps != 500.5 {
		t.Errorf("download_mbps = %v, want 500.5", result.DownloadMbps)
	}
	if result.ServerName != "Test" {
		t.Errorf("server_name = %q, want Test", result.ServerName)
	}
}
