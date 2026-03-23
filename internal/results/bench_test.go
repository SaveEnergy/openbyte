package results

import (
	"encoding/json"
	"testing"
	"time"
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

// BenchmarkMarshalResult matches JSON work for GET /results/{id} responses (typical row shape).
func BenchmarkMarshalResult(b *testing.B) {
	r := Result{
		ID:               "a1b2c3d4",
		DownloadMbps:     942.7,
		UploadMbps:       88.3,
		LatencyMs:        12.4,
		JitterMs:         0.8,
		LoadedLatencyMs:  18.1,
		BufferbloatGrade: "B",
		IPv4:             "192.0.2.10",
		IPv6:             "",
		ServerName:       "bench-east",
		CreatedAt:        time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, err := json.Marshal(r)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidResultIDMatch is the regexp gate on results routes (GET/PUT paths).
func BenchmarkValidResultIDMatch(b *testing.B) {
	id := "a1b2c3d4"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !validID.MatchString(id) {
			b.Fatal("expected match")
		}
	}
}

// BenchmarkValidResultIDReject catches malformed IDs cheaply.
func BenchmarkValidResultIDReject(b *testing.B) {
	id := "not-valid-id-too-long"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if validID.MatchString(id) {
			b.Fatal("expected reject")
		}
	}
}

// BenchmarkStoreSave is INSERT throughput on the results SQLite path (:memory:).
func BenchmarkStoreSave(b *testing.B) {
	s := benchOpenStore(b)
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
		if _, err := s.Save(r); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStoreGetByID is repeated SELECT for a fixed row (Save once outside the timer).
func BenchmarkStoreGetByID(b *testing.B) {
	s := benchOpenStore(b)
	id, err := s.Save(Result{
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
		got, err := s.Get(id)
		if err != nil || got == nil {
			b.Fatalf("get: err=%v got=%v", err, got)
		}
	}
}
