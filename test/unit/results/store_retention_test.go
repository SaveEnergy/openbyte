package results_test

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/results"
	_ "modernc.org/sqlite" // Registers sqlite driver for direct sql.Open assertions.
)

func TestStoreTrimToMax(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "trim.db")

	// Create store with max 3 results
	store, err := results.New(dbPath, 3)
	if err != nil {
		t.Fatalf(storeNewErrFmt, err)
	}

	ids := make([]string, 5)
	for i := range 5 {
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
		t.Fatalf(storeReopenFmt, err)
	}
	defer store2.Close()

	// Oldest 2 should be gone
	for _, id := range ids[:2] {
		got, getErr := store2.Get(id)
		if getErr != nil {
			t.Fatalf(storeGetByIDFmt, id, getErr)
		}
		if got != nil {
			t.Errorf(storeTrimmedErrFmt, id)
		}
	}
	// Newest 3 should remain
	for _, id := range ids[2:] {
		got, getErr := store2.Get(id)
		if getErr != nil {
			t.Fatalf(storeGetByIDFmt, id, getErr)
		}
		if got == nil {
			t.Errorf(storeRemainErrFmt, id)
		}
	}
}

func TestStoreTrimToMaxDeterministicWithEqualCreatedAt(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "trim-deterministic.db")

	store, err := results.New(dbPath, 2)
	if err != nil {
		t.Fatalf(storeNewErrFmt, err)
	}
	store.Close()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf(storeOpenSQLiteFmt, err)
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
		t.Fatalf(storeReopenFmt, err)
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
		t.Fatalf(storeKeptBBBBMsg)
	}
	keptB, err := store2.Get("CCCC0001")
	if err != nil {
		t.Fatalf("Get kept id CCCC0001: %v", err)
	}
	if keptB == nil {
		t.Fatalf(storeKeptCCCCMsg)
	}
}
