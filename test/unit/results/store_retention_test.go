package results_test

import (
	"context"
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
		id, saveErr := store.Save(context.Background(), results.Result{
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

	// Rapid saves can share a created_at on coarse-timer platforms (Windows),
	// making the trim tie-break keep rows by random id instead of insertion
	// order. Assign strictly increasing timestamps so "oldest" is well-defined.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf(storeOpenSQLiteFmt, err)
	}
	base := time.Now().UTC().Add(-time.Minute)
	for i, id := range ids {
		if _, execErr := db.Exec(`UPDATE results SET created_at = ? WHERE id = ?`,
			base.Add(time.Duration(i)*time.Second), id); execErr != nil {
			t.Fatalf("set created_at for %s: %v", id, execErr)
		}
	}
	db.Close()

	// Reopen — cleanup runs on startup, should trim to 3
	store2, err := results.New(dbPath, 3)
	if err != nil {
		t.Fatalf(storeReopenFmt, err)
	}
	defer store2.Close()

	// Oldest 2 should be gone
	for _, id := range ids[:2] {
		got, getErr := store2.Get(context.Background(), id)
		if getErr != nil {
			t.Fatalf(storeGetByIDFmt, id, getErr)
		}
		if got != nil {
			t.Errorf(storeTrimmedErrFmt, id)
		}
	}
	// Newest 3 should remain
	for _, id := range ids[2:] {
		got, getErr := store2.Get(context.Background(), id)
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

	// Use a recent timestamp so age-based retention (90 days) does not delete rows
	// before max-count trimming is exercised.
	ts := time.Now().UTC().Truncate(time.Second)
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

	trimmed, err := store2.Get(context.Background(), "AAAA0001")
	if err != nil {
		t.Fatalf("Get trimmed id: %v", err)
	}
	if trimmed != nil {
		t.Fatal("expected AAAA0001 trimmed under deterministic tie-break")
	}
	keptA, err := store2.Get(context.Background(), "BBBB0001")
	if err != nil {
		t.Fatalf("Get kept id BBBB0001: %v", err)
	}
	if keptA == nil {
		t.Fatal(storeKeptBBBBMsg)
	}
	keptB, err := store2.Get(context.Background(), "CCCC0001")
	if err != nil {
		t.Fatalf("Get kept id CCCC0001: %v", err)
	}
	if keptB == nil {
		t.Fatal(storeKeptCCCCMsg)
	}
}
