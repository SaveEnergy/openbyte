package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkIsJSONContentType(b *testing.B) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !isJSONContentType(req) {
			b.Fatal("expected json")
		}
	}
}
