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
	dbPath := filepath.Join(tempDir, resultsDBName)

	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf(newStoreFmt, err)
	}

	// Force save path to fail by closing DB before handler call.
	store.Close()

	h := results.NewHandler(store)

	body := sampleResultPayload

	req := httptest.NewRequest(http.MethodPost, resultsPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()

	h.Save(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusInternalServerError)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf(decodeResponseFmt, err)
	}
	if got := resp["error"]; got != saveErrorMsg {
		t.Fatalf(errorWantFmt, got, saveErrorMsg)
	}

	_ = os.Remove(dbPath)
}
func TestSaveSucceedsReturns201WithIDAndURL(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, resultsDBName)

	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf(newStoreFmt, err)
	}
	defer store.Close()

	h := results.NewHandler(store)
	body := sampleResultPayload
	req := httptest.NewRequest(http.MethodPost, resultsPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	h.Save(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusCreated)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf(decodeResponseFmt, err)
	}
	id := resp["id"]
	url := resp["url"]
	if len(id) != 8 {
		t.Fatalf("id = %q, want 8-char id", id)
	}
	if url != "/results/"+id {
		t.Fatalf("url = %q, want %q", url, "/results/"+id)
	}
	if got := rec.Header().Get(cacheControlHeader); got != cacheNoStore {
		t.Fatalf(cacheControlWantFmt, got, cacheNoStore)
	}
}

func TestHandlerSaveRejectsWrongContentTypeDrainsBody(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, resultsDBName)
	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf(newStoreFmt, err)
	}
	defer store.Close()

	h := results.NewHandler(store)
	tb := &trackingBody{data: []byte(`{"download_mbps":1}`)}
	req := httptest.NewRequest(http.MethodPost, resultsPath, nil)
	req.Body = tb
	req.Header.Set(contentTypeHeader, plainTextType)
	rec := httptest.NewRecorder()

	h.Save(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusUnsupportedMediaType)
	}
	if tb.reads == 0 {
		t.Fatal("expected body to be drained")
	}
	if !tb.closed {
		t.Fatal("expected body to be closed")
	}
}

func TestSaveWithWriteFailureStillSetsCreatedStatus(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, resultsDBName)
	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf(newStoreFmt, err)
	}
	defer store.Close()

	h := results.NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, resultsPath, strings.NewReader(sampleResultPayload))
	req.Header.Set(contentTypeHeader, applicationJSON)
	fw := &failingResponseWriter{}

	h.Save(fw, req)
	if fw.status != http.StatusCreated {
		t.Fatalf(statusCodeWantFmt, fw.status, http.StatusCreated)
	}
	if fw.writes == 0 {
		t.Fatal("expected write to be attempted")
	}
}

func TestHandlerSaveBodyTooLarge(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, resultsDBName)
	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf(newStoreFmt, err)
	}
	defer store.Close()

	h := results.NewHandler(store)
	large := strings.Repeat("x", 5000)
	body := `{"download_mbps":1,"upload_mbps":1,"latency_ms":1,"jitter_ms":1,"loaded_latency_ms":1,"bufferbloat_grade":"A","ipv4":"203.0.113.10","ipv6":"","server_name":"` + large + `"}`

	req := httptest.NewRequest(http.MethodPost, resultsPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()

	h.Save(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusRequestEntityTooLarge)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf(decodeResponseFmt, err)
	}
	if got := resp["error"]; got != bodyTooLargeMsg {
		t.Fatalf(errorWantFmt, got, bodyTooLargeMsg)
	}
}

func TestSaveAcceptsOptionalDiagnostics(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, resultsDBName)
	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf(newStoreFmt, err)
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
		"server_name": "test",
		"diagnostics": {"download": {"peakMbps": 105, "sustainedMbps": 98, "volatility": 2.1, "stopReason": "duration"}}
	}`
	req := httptest.NewRequest(http.MethodPost, resultsPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	h.Save(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusCreated)
	}
}

func TestHandlerSaveRejectsUnknownFields(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, resultsDBName)
	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf(newStoreFmt, err)
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
		"server_name": "test",
		"unknown": 1
	}`

	req := httptest.NewRequest(http.MethodPost, resultsPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	h.Save(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusBadRequest)
	}
	if got := rec.Header().Get(cacheControlHeader); got != cacheNoStore {
		t.Fatalf(cacheControlWantFmt, got, cacheNoStore)
	}
}
