package results_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/results"
)

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
		t.Fatalf(decodeResponseFmt, err)
	}
	if got := resp["error"]; got != internalErrorMsg {
		t.Fatalf(errorWantFmt, got, internalErrorMsg)
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
