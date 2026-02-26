package results_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/results"
)

type failingResponseWriter struct {
	header http.Header
	status int
	writes int
}

const (
	resultsPath         = "/api/v1/results"
	resultsDBName       = "results.db"
	newStoreFmt         = "new store: %v"
	contentTypeHeader   = "Content-Type"
	applicationJSON     = "application/json"
	cacheControlHeader  = "Cache-Control"
	cacheNoStore        = "no-store"
	statusCodeWantFmt   = "status = %d, want %d"
	cacheControlWantFmt = "cache-control = %q, want %q"
	plainTextType       = "text/plain"
	abcResultID         = "abc12345"
	invalidResultID     = "bad-id"
	missingResultID     = "missing1"
	saveErrorMsg        = "failed to save result"
	internalErrorMsg    = "internal error"
	bodyTooLargeMsg     = "request body too large"
	sampleResultPayload = `{
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
)

func (fw *failingResponseWriter) Header() http.Header {
	if fw.header == nil {
		fw.header = make(http.Header)
	}
	return fw.header
}

func (fw *failingResponseWriter) WriteHeader(code int) {
	fw.status = code
}

func (fw *failingResponseWriter) Write(_ []byte) (int, error) {
	fw.writes++
	return 0, errors.New("write failed")
}

type trackingBody struct {
	data   []byte
	offset int
	reads  int
	closed bool
}

func (tb *trackingBody) Read(p []byte) (int, error) {
	tb.reads++
	if tb.offset >= len(tb.data) {
		return 0, io.EOF
	}
	n := copy(p, tb.data[tb.offset:])
	tb.offset += n
	return n, nil
}

func (tb *trackingBody) Close() error {
	tb.closed = true
	return nil
}

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
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["error"]; got != saveErrorMsg {
		t.Fatalf("error = %q, want %q", got, saveErrorMsg)
	}

	_ = os.Remove(dbPath)
}

func TestGetReturnsNoStoreForSavedResult(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, resultsDBName)

	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf(newStoreFmt, err)
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
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(cacheControlHeader); got != cacheNoStore {
		t.Fatalf(cacheControlWantFmt, got, cacheNoStore)
	}
}

func TestGetReturnsNotFoundForMissingResult(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, resultsDBName)

	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf(newStoreFmt, err)
	}
	defer store.Close()

	h := results.NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, resultsPath+"/"+abcResultID, nil)
	req.SetPathValue("id", abcResultID)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusNotFound)
	}
}

func TestGetRejectsInvalidResultID(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, resultsDBName)

	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf(newStoreFmt, err)
	}
	defer store.Close()

	h := results.NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, resultsPath+"/"+invalidResultID, nil)
	req.SetPathValue("id", invalidResultID)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusBadRequest)
	}
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

func TestGetReturnsInternalErrorWhenStoreFails(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, resultsDBName)

	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf(newStoreFmt, err)
	}
	store.Close()

	h := results.NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, resultsPath+"/"+abcResultID, nil)
	req.SetPathValue("id", abcResultID)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusInternalServerError)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["error"]; got != internalErrorMsg {
		t.Fatalf("error = %q, want %q", got, internalErrorMsg)
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
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["error"]; got != bodyTooLargeMsg {
		t.Fatalf("error = %q, want %q", got, bodyTooLargeMsg)
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

func TestResultsResponsesSetNoStore(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, resultsDBName)
	store, err := results.New(dbPath, 100)
	if err != nil {
		t.Fatalf(newStoreFmt, err)
	}
	defer store.Close()

	h := results.NewHandler(store)

	saveReq := httptest.NewRequest(http.MethodPost, resultsPath, strings.NewReader(sampleResultPayload))
	saveReq.Header.Set(contentTypeHeader, applicationJSON)
	saveRec := httptest.NewRecorder()
	h.Save(saveRec, saveReq)
	if saveRec.Header().Get(cacheControlHeader) != cacheNoStore {
		t.Fatalf("save cache-control = %q, want %q", saveRec.Header().Get(cacheControlHeader), cacheNoStore)
	}

	getReq := httptest.NewRequest(http.MethodGet, resultsPath+"/"+missingResultID, nil)
	getReq.SetPathValue("id", missingResultID)
	getRec := httptest.NewRecorder()
	h.Get(getRec, getReq)
	if getRec.Header().Get(cacheControlHeader) != cacheNoStore {
		t.Fatalf("get cache-control = %q, want %q", getRec.Header().Get(cacheControlHeader), cacheNoStore)
	}
}
