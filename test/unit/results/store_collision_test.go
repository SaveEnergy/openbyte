package results_test

import (
	"context"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/results"
)

func TestStoreSaveRetriesIDCollision(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()
	useIDEntropy(t, 0, 0, 1)

	ctx := context.Background()
	originalID, err := store.Save(ctx, results.Result{DownloadMbps: 10})
	if err != nil {
		t.Fatalf("save original result: %v", err)
	}
	newID, err := store.Save(ctx, results.Result{DownloadMbps: 20})
	if err != nil {
		t.Fatalf("save after ID collision: %v", err)
	}
	if newID == originalID {
		t.Fatalf("save reused existing ID %q", newID)
	}

	for id, wantMbps := range map[string]float64{originalID: 10, newID: 20} {
		got, err := store.Get(ctx, id)
		if err != nil {
			t.Fatalf("get %s: %v", id, err)
		}
		if got == nil || got.DownloadMbps != wantMbps {
			t.Errorf("result %s = %+v, want download_mbps %v", id, got, wantMbps)
		}
	}
}

func TestStoreSaveRejectsExhaustedIDCollisions(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()
	entropy := useIDEntropy(t, 0)

	ctx := context.Background()
	originalID, err := store.Save(ctx, results.Result{DownloadMbps: 10})
	if err != nil {
		t.Fatalf("save original result: %v", err)
	}
	entropy.reads = 0
	id, err := store.Save(ctx, results.Result{DownloadMbps: 20})
	if err == nil || !strings.Contains(err.Error(), "failed to generate unique ID after 5 attempts") {
		t.Fatalf("save error = %v, want exhausted ID retries", err)
	}
	if id != "" {
		t.Errorf("failed save returned ID %q", id)
	}
	if entropy.reads != 5 {
		t.Errorf("ID generation attempts = %d, want 5", entropy.reads)
	}
	got, err := store.Get(ctx, originalID)
	if err != nil {
		t.Fatalf("get original result: %v", err)
	}
	if got == nil || got.DownloadMbps != 10 {
		t.Fatalf("original result = %+v, want download_mbps 10", got)
	}
}

// These tests must remain sequential: crypto/rand.Reader is process-wide.
func useIDEntropy(t *testing.T, values ...byte) *idEntropyReader {
	t.Helper()
	original := rand.Reader
	t.Cleanup(func() { rand.Reader = original })
	reader := &idEntropyReader{values: values}
	rand.Reader = reader
	return reader
}

type idEntropyReader struct {
	values []byte
	reads  int
}

func (r *idEntropyReader) Read(p []byte) (int, error) {
	// Repeat the last value so extra retries never exhaust the entropy source.
	value := r.values[min(r.reads, len(r.values)-1)]
	r.reads++
	for i := range p {
		p[i] = value
	}
	return len(p), nil
}
