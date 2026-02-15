package results_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"encoding/json"

	"github.com/saveenergy/openbyte/internal/results"
	_ "modernc.org/sqlite"
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

func TestStoreTrimToMaxDeterministicWithEqualCreatedAt(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "trim-deterministic.db")

	store, err := results.New(dbPath, 2)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	store.Close()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	ts := time.Date(2026, 2, 15, 6, 0, 0, 0, time.UTC)
	ids := []string{"AAAA0001", "BBBB0001", "CCCC0001"}
	for _, id := range ids {
		_, execErr := db.Exec(
			`INSERT INTO results (id, download_mbps, upload_mbps, latency_ms, jitter_ms, loaded_latency_ms, bufferbloat_grade, ipv4, ipv6, server_name, created_at)
			VALUES (?, 1, 1, 1, 0, 0, '', '', '', '', ?)`,
			id, ts,
		)
		if execErr != nil {
			t.Fatalf("insert %s: %v", id, execErr)
		}
	}

	store2, err := results.New(dbPath, 2)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer store2.Close()

	trimmed, err := store2.Get("AAAA0001")
	if err != nil {
		t.Fatalf("Get trimmed id: %v", err)
	}
	if trimmed != nil {
		t.Fatalf("expected AAAA0001 trimmed under deterministic tie-break")
	}
	keptA, err := store2.Get("BBBB0001")
	if err != nil {
		t.Fatalf("Get kept id BBBB0001: %v", err)
	}
	if keptA == nil {
		t.Fatalf("expected BBBB0001 kept")
	}
	keptB, err := store2.Get("CCCC0001")
	if err != nil {
		t.Fatalf("Get kept id CCCC0001: %v", err)
	}
	if keptB == nil {
		t.Fatalf("expected CCCC0001 kept")
	}
}

func TestStoreSaveRetriesOnBusyError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "busy-retry.db")

	store, err := results.New(dbPath, 10)
	if err != nil {
		t.Fatalf("New store: %v", err)
	}
	defer store.Close()

	lockDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open lock db: %v", err)
	}
	defer lockDB.Close()

	if _, err := lockDB.Exec("PRAGMA busy_timeout=5000"); err != nil {
		t.Fatalf("set lock busy_timeout: %v", err)
	}
	if _, err := lockDB.Exec("BEGIN EXCLUSIVE"); err != nil {
		t.Fatalf("begin exclusive: %v", err)
	}
	defer lockDB.Exec("ROLLBACK")

	released := make(chan struct{})
	go func() {
		time.Sleep(7 * time.Second)
		_, _ = lockDB.Exec("COMMIT")
		close(released)
	}()

	start := time.Now()
	id, saveErr := store.Save(results.Result{
		DownloadMbps: 10,
		UploadMbps:   5,
		LatencyMs:    12,
		JitterMs:     1,
	})
	elapsed := time.Since(start)
	if saveErr != nil {
		t.Fatalf("Save after busy lock: %v", saveErr)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}
	<-released
	if elapsed < 5*time.Second {
		t.Fatalf("expected busy lock to delay save, elapsed=%v", elapsed)
	}
}

func TestStoreGetRetriesOnBusyError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "busy-read-retry.db")

	store, err := results.New(dbPath, 10)
	if err != nil {
		t.Fatalf("New store: %v", err)
	}
	defer store.Close()

	id, err := store.Save(results.Result{DownloadMbps: 10, UploadMbps: 5, LatencyMs: 12, JitterMs: 1})
	if err != nil {
		t.Fatalf("save seed result: %v", err)
	}

	lockDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open lock db: %v", err)
	}
	defer lockDB.Close()
	if _, err := lockDB.Exec("PRAGMA busy_timeout=5000"); err != nil {
		t.Fatalf("set lock busy_timeout: %v", err)
	}
	if _, err := lockDB.Exec("BEGIN EXCLUSIVE"); err != nil {
		t.Fatalf("begin exclusive: %v", err)
	}
	defer lockDB.Exec("ROLLBACK")

	released := make(chan struct{})
	go func() {
		time.Sleep(7 * time.Second)
		_, _ = lockDB.Exec("COMMIT")
		close(released)
	}()

	start := time.Now()
	got, getErr := store.Get(id)
	_ = time.Since(start)
	if getErr != nil {
		t.Fatalf("Get after busy lock: %v", getErr)
	}
	if got == nil {
		t.Fatal("expected result after lock release")
	}
	<-released
}

func TestStoreCleanupRetriesOnBusyError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "busy-cleanup-retry.db")

	store, err := results.New(dbPath, 10)
	if err != nil {
		t.Fatalf("New store: %v", err)
	}

	id, err := store.Save(results.Result{DownloadMbps: 10, UploadMbps: 5, LatencyMs: 12, JitterMs: 1})
	if err != nil {
		t.Fatalf("save seed result: %v", err)
	}
	store.Close()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	old := time.Now().Add(-100 * 24 * time.Hour).UTC()
	if _, err := db.Exec(`UPDATE results SET created_at = ? WHERE id = ?`, old, id); err != nil {
		t.Fatalf("backdate result: %v", err)
	}
	db.Close()

	lockDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open lock db: %v", err)
	}
	defer lockDB.Close()
	if _, err := lockDB.Exec("PRAGMA busy_timeout=5000"); err != nil {
		t.Fatalf("set lock busy_timeout: %v", err)
	}
	if _, err := lockDB.Exec("BEGIN EXCLUSIVE"); err != nil {
		t.Fatalf("begin exclusive: %v", err)
	}
	defer lockDB.Exec("ROLLBACK")

	released := make(chan struct{})
	go func() {
		time.Sleep(7 * time.Second)
		_, _ = lockDB.Exec("COMMIT")
		close(released)
	}()

	reopenStart := time.Now()
	store2, err := results.New(dbPath, 10)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer store2.Close()
	<-released
	if time.Since(reopenStart) < 5*time.Second {
		t.Fatalf("expected cleanup to block on lock and retry")
	}

	trimmed, err := store2.Get(id)
	if err != nil {
		t.Fatalf("get trimmed result: %v", err)
	}
	if trimmed != nil {
		t.Fatalf("expected old result to be deleted by cleanup retry")
	}
}

func TestHandlerSaveValidation(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	handler := results.NewHandler(store)
	router := http.NewServeMux()
	router.HandleFunc("POST /api/v1/results", handler.Save)
	router.HandleFunc("GET /api/v1/results/{id}", handler.Get)

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"valid", `{"download_mbps":100,"upload_mbps":50,"latency_ms":10,"jitter_ms":1}`, http.StatusCreated},
		{"negative download", `{"download_mbps":-1,"upload_mbps":50,"latency_ms":10,"jitter_ms":1}`, http.StatusBadRequest},
		{"out of range", `{"download_mbps":200000,"upload_mbps":50,"latency_ms":10,"jitter_ms":1}`, http.StatusBadRequest},
		{"multiple json objects", `{"download_mbps":100}{"upload_mbps":50}`, http.StatusBadRequest},
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
	router := http.NewServeMux()
	router.HandleFunc("GET /api/v1/results/{id}", handler.Get)

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

func TestHandlerSaveRejectsWrongContentType(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	handler := results.NewHandler(store)
	router := http.NewServeMux()
	router.HandleFunc("POST /api/v1/results", handler.Save)

	body := `{"download_mbps":100,"upload_mbps":50,"latency_ms":10,"jitter_ms":1}`
	req := httptest.NewRequest("POST", "/api/v1/results", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain: status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}

	// Verify the error response is JSON
	var errResp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Errorf("error response not JSON: %v", err)
	}
	if errResp["error"] == "" {
		t.Error("expected error field in response")
	}
}

func TestHandlerRoundTrip(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	handler := results.NewHandler(store)
	router := http.NewServeMux()
	router.HandleFunc("POST /api/v1/results", handler.Save)
	router.HandleFunc("GET /api/v1/results/{id}", handler.Get)

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
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want %q", got, "no-store")
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

func TestStoreCloseIdempotent(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	store.Close()
	store.Close()
}

func TestGenerateIDUsesValidCharset(t *testing.T) {
	store, cleanup := tempStore(t, 10000)
	defer cleanup()

	const samples = 2000
	const idCharset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	seen := make(map[rune]struct{})

	for i := 0; i < samples; i++ {
		id, err := store.Save(results.Result{
			DownloadMbps: 1, UploadMbps: 1, LatencyMs: 1, JitterMs: 1,
		})
		if err != nil {
			t.Fatalf("save result: %v", err)
		}
		for _, ch := range id {
			if !strings.ContainsRune(idCharset, ch) {
				t.Fatalf("id has invalid char %q in %q", ch, id)
			}
			seen[ch] = struct{}{}
		}
	}

	if len(seen) != len(idCharset) {
		t.Fatalf("seen charset size = %d, want %d", len(seen), len(idCharset))
	}
}
