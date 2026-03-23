package registry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/logging"
)

// BenchmarkRegistryAuthenticateOK is constant-time bearer comparison + header parsing (mutations path).
func BenchmarkRegistryAuthenticateOK(b *testing.B) {
	const key = "registry-api-key-32bytes-long!!"
	h := NewHandler(nil, logging.GetLogger(), key)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/registry/servers", nil)
	req.Header.Set("Authorization", authBearerPrefix+key)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !h.authenticate(req) {
			b.Fatal("expected auth ok")
		}
	}
}

// BenchmarkRegistryAuthenticateWrongToken exercises the rejection path (still constant-time compare length).
func BenchmarkRegistryAuthenticateWrongToken(b *testing.B) {
	key := strings.Repeat("a", 48)
	h := NewHandler(nil, logging.GetLogger(), key)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/registry/servers", nil)
	req.Header.Set("Authorization", authBearerPrefix+strings.Repeat("b", 48))

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if h.authenticate(req) {
			b.Fatal("expected auth fail")
		}
	}
}
