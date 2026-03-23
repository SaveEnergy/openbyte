package api

import (
	"errors"
	"net/http"
	"testing"

	pkgerrors "github.com/saveenergy/openbyte/pkg/errors"
)

// BenchmarkRespondErrorStreamError uses StreamError unwrap (common API error path).
func BenchmarkRespondErrorStreamError(b *testing.B) {
	err := pkgerrors.ErrStreamNotFound("550e8400-e29b-41d4-a716-446655440000")
	var w benchJSONWriter

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		w.buf.Reset()
		if w.hdr != nil {
			for k := range w.hdr {
				delete(w.hdr, k)
			}
		}
		respondError(&w, err, http.StatusNotFound)
	}
}

// BenchmarkRespondErrorGeneric uses plain error stringification.
func BenchmarkRespondErrorGeneric(b *testing.B) {
	err := errors.New("upstream failure")
	var w benchJSONWriter

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		w.buf.Reset()
		if w.hdr != nil {
			for k := range w.hdr {
				delete(w.hdr, k)
			}
		}
		respondError(&w, err, http.StatusBadGateway)
	}
}
