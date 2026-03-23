package api

import "testing"

// BenchmarkIsValidStreamIDValid is the hot path for every /streams/{id} request with a real UUID.
func BenchmarkIsValidStreamIDValid(b *testing.B) {
	id := "550e8400-e29b-41d4-a716-446655440000"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !isValidStreamID(id) {
			b.Fatal("expected valid")
		}
	}
}

// BenchmarkIsValidStreamIDInvalid rejects bad IDs without accepting malformed input.
func BenchmarkIsValidStreamIDInvalid(b *testing.B) {
	id := "not-a-real-uuid-at-all"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if isValidStreamID(id) {
			b.Fatal("expected invalid")
		}
	}
}
