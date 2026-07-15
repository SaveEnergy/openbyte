package results

import (
	"context"
	"testing"
)

// Open test store without long-lived cleanup noise for microbenches.
func benchOpenStore(b *testing.B) *Store {
	b.Helper()
	s, err := New(":memory:", 100_000)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { s.Close() })
	return s
}

// BenchmarkStoreSave is INSERT throughput on the results SQLite path (:memory:).
func BenchmarkStoreSave(b *testing.B) {
	s := benchOpenStore(b)
	ctx := context.Background()
	r := Result{
		DownloadMbps:     100,
		UploadMbps:       20,
		LatencyMs:        10,
		JitterMs:         0.5,
		LoadedLatencyMs:  12,
		BufferbloatGrade: "A",
		IPv4:             "192.0.2.1",
		ServerName:       "bench",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := s.Save(ctx, r); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStoreGetByID is repeated SELECT for a fixed row (Save once outside the timer).
func BenchmarkStoreGetByID(b *testing.B) {
	s := benchOpenStore(b)
	ctx := context.Background()
	id, err := s.Save(ctx, Result{
		DownloadMbps:     200,
		UploadMbps:       40,
		LatencyMs:        8,
		JitterMs:         0.2,
		LoadedLatencyMs:  9,
		BufferbloatGrade: "B",
		IPv4:             "192.0.2.2",
		ServerName:       "bench-get",
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		got, err := s.Get(ctx, id)
		if err != nil || got == nil {
			b.Fatalf("get: err=%v got=%v", err, got)
		}
	}
}
