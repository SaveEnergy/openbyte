package results_test

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/results"
	_ "modernc.org/sqlite" // Registers sqlite driver for direct sql.Open assertions.
)

func TestStoreSaveRetriesOnBusyError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "busy-retry.db")

	store, err := results.New(dbPath, 10)
	if err != nil {
		t.Fatalf(storeNewFmt, err)
	}
	defer store.Close()

	lockDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf(storeOpenLockDBFmt, err)
	}
	defer lockDB.Close()

	if _, err := lockDB.Exec(storeBusyTimeoutSQL); err != nil {
		t.Fatalf(storeSetBusyTimeoutFmt, err)
	}
	if _, err := lockDB.Exec(storeBeginExclusiveSQL); err != nil {
		t.Fatalf(storeBeginExclusiveFmt, err)
	}
	defer lockDB.Exec(storeRollbackSQL)

	released := make(chan struct{})
	go func() {
		time.Sleep(storeLockReleaseDelay)
		_, _ = lockDB.Exec(storeCommitSQL)
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
		t.Fatal(storeExpectedNonEmptyID)
	}
	<-released
	if elapsed < storeMinLockWait {
		t.Fatalf("expected busy lock to delay save, elapsed=%v", elapsed)
	}
}

func TestStoreGetRetriesOnBusyError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "busy-read-retry.db")

	store, err := results.New(dbPath, 10)
	if err != nil {
		t.Fatalf(storeNewFmt, err)
	}
	defer store.Close()

	id, err := store.Save(results.Result{DownloadMbps: 10, UploadMbps: 5, LatencyMs: 12, JitterMs: 1})
	if err != nil {
		t.Fatalf(storeSaveSeedFmt, err)
	}

	lockDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf(storeOpenLockDBFmt, err)
	}
	defer lockDB.Close()
	if _, err := lockDB.Exec(storeBusyTimeoutSQL); err != nil {
		t.Fatalf(storeSetBusyTimeoutFmt, err)
	}
	if _, err := lockDB.Exec(storeBeginExclusiveSQL); err != nil {
		t.Fatalf(storeBeginExclusiveFmt, err)
	}
	defer lockDB.Exec(storeRollbackSQL)

	released := make(chan struct{})
	go func() {
		time.Sleep(storeLockReleaseDelay)
		_, _ = lockDB.Exec(storeCommitSQL)
		close(released)
	}()

	start := time.Now()
	got, getErr := store.Get(id)
	_ = time.Since(start)
	if getErr != nil {
		t.Fatalf("Get after busy lock: %v", getErr)
	}
	if got == nil {
		t.Fatal(storeExpectedResultAfterLock)
	}
	<-released
}

func TestStoreCleanupRetriesOnBusyError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "busy-cleanup-retry.db")

	store, err := results.New(dbPath, 10)
	if err != nil {
		t.Fatalf(storeNewFmt, err)
	}

	id, err := store.Save(results.Result{DownloadMbps: 10, UploadMbps: 5, LatencyMs: 12, JitterMs: 1})
	if err != nil {
		t.Fatalf(storeSaveSeedFmt, err)
	}
	store.Close()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf(storeOpenSQLiteFmt, err)
	}
	old := time.Now().Add(-100 * 24 * time.Hour).UTC()
	if _, err := db.Exec(`UPDATE results SET created_at = ? WHERE id = ?`, old, id); err != nil {
		t.Fatalf("backdate result: %v", err)
	}
	db.Close()

	lockDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf(storeOpenLockDBFmt, err)
	}
	defer lockDB.Close()
	if _, err := lockDB.Exec(storeBusyTimeoutSQL); err != nil {
		t.Fatalf(storeSetBusyTimeoutFmt, err)
	}
	if _, err := lockDB.Exec(storeBeginExclusiveSQL); err != nil {
		t.Fatalf(storeBeginExclusiveFmt, err)
	}
	defer lockDB.Exec(storeRollbackSQL)

	released := make(chan struct{})
	go func() {
		time.Sleep(storeLockReleaseDelay)
		_, _ = lockDB.Exec(storeCommitSQL)
		close(released)
	}()

	reopenStart := time.Now()
	store2, err := results.New(dbPath, 10)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer store2.Close()
	<-released
	if time.Since(reopenStart) < storeMinLockWait {
		t.Fatal("expected cleanup to block on lock and retry")
	}

	trimmed, err := store2.Get(id)
	if err != nil {
		t.Fatalf("get trimmed result: %v", err)
	}
	if trimmed != nil {
		t.Fatal(storeExpectedCleanupDeleteMsg)
	}
}
