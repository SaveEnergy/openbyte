package api

import (
	"net/http"
	"testing"
	"time"
)

func BenchmarkShouldSkipRequestLog(b *testing.B) {
	path := "/api/v1/ping"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !shouldSkipRequestLog(path) {
			b.Fatal("expected skip")
		}
	}
}

func BenchmarkShouldLogRequestAPIOK(b *testing.B) {
	path := "/api/v1/results/abc12345"
	duration := 50 * time.Millisecond

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !shouldLogRequest(path, http.StatusOK, duration) {
			b.Fatal("expected log")
		}
	}
}

func BenchmarkShouldLogRequestUploadFast(b *testing.B) {
	path := "/api/v1/upload"
	duration := 10 * time.Millisecond

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if shouldLogRequest(path, http.StatusOK, duration) {
			b.Fatal("fast 200 upload should not log")
		}
	}
}

func BenchmarkShouldLogRequestUploadSlow(b *testing.B) {
	path := "/api/v1/upload"
	duration := 2 * time.Second

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !shouldLogRequest(path, http.StatusOK, duration) {
			b.Fatal("slow upload should log")
		}
	}
}
