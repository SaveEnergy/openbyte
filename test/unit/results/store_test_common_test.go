package results_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/results"
)

const (
	storeNewFmt                   = "New store: %v"
	storeGetFmt                   = "Get: %v"
	storeReopenFmt                = "Reopen: %v"
	storeOpenSQLiteFmt            = "open sqlite: %v"
	storeSaveSeedFmt              = "save seed result: %v"
	storeOpenLockDBFmt            = "open lock db: %v"
	storeSetBusyTimeoutFmt        = "set lock busy_timeout: %v"
	storeBeginExclusiveFmt        = "begin exclusive: %v"
	storeNewErrFmt                = "New: %v"
	storeGetByIDFmt               = "Get %s: %v"
	resultsAPIPath                = "/api/v1/results"
	resultsAPIBasePath            = "/api/v1/results/"
	jsonContentType               = "application/json"
	storeContentTypeHeader        = "Content-Type"
	storeCacheControlHeader       = "Cache-Control"
	storeHTTPMethodGet            = "GET"
	storeHTTPMethodPost           = "POST"
	storeTextPlainContentType     = "text/plain"
	storeStatusWantFmt            = "status = %d, want %d"
	storeStatusWithBodyFmt        = "%s status = %d, want %d; body: %s"
	storeTextPlainStatusFmt       = "text/plain: status = %d, want %d"
	storeCacheControlFmt          = "cache-control = %q, want %q"
	storeDecodeResponseFmt        = "decode %s response: %v"
	storeSaveAction               = "save"
	storeGetAction                = "get"
	storeSaveFmt                  = "Save: %v"
	storeExpectedIDLenFmt         = "expected 8-char ID, got %q"
	storeGetReturnedNilMsg        = "Get returned nil"
	storeDownloadMbpsFmt          = "download_mbps = %v, want 123.4"
	storeUploadMbpsFmt            = "upload_mbps = %v, want 56.7"
	storeIPv4Fmt                  = "ipv4 = %q, want 1.2.3.4"
	storeBufferbloatFmt           = "bufferbloat_grade = %q, want A"
	storeExpectedNonEmptyID       = "expected non-empty id"
	storeExpectedResultAfterLock  = "expected result after lock release"
	storeExpectedCleanupDeleteMsg = "expected old result to be deleted by cleanup retry"
	storeErrorResponseJSONFmt     = "error response not JSON: %v"
	storeExpectedErrorField       = "expected error field in response"
	storeEmptyIDMsg               = "empty ID in response"
	storeSaveResultFmt            = "save result: %v"
	resultsPostRoute              = "POST /api/v1/results"
	resultsGetRoute               = "GET /api/v1/results/{id}"
	storeBusyTimeoutSQL           = "PRAGMA busy_timeout=5000"
	storeBeginExclusiveSQL        = "BEGIN EXCLUSIVE"
	storeRollbackSQL              = "ROLLBACK"
	storeCommitSQL                = "COMMIT"
	storeLockReleaseDelay         = 7 * time.Second
	storeMinLockWait              = 5 * time.Second
	storeCacheNoStore             = "no-store"
	storeTrimmedErrFmt            = "expected id %s to be trimmed, but found it"
	storeRemainErrFmt             = "expected id %s to remain, but not found"
	storeKeptBBBBMsg              = "expected BBBB0001 kept"
	storeKeptCCCCMsg              = "expected CCCC0001 kept"
)

func tempStore(t *testing.T, maxResults int) (*results.Store, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := results.New(dbPath, maxResults)
	if err != nil {
		t.Fatalf(storeNewFmt, err)
	}
	return s, func() { s.Close(); os.RemoveAll(dir) }
}
