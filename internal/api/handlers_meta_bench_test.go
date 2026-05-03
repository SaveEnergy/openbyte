package api

import (
	"encoding/json"
	"testing"
)

// BenchmarkMarshalVersionResponse matches GET /api/v1/version payload.
func BenchmarkMarshalVersionResponse(b *testing.B) {
	v := VersionResponse{Version: "0.0.0+bench", ServerName: "bench-server"}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, err := json.Marshal(v)
		if err != nil {
			b.Fatal(err)
		}
	}
}
