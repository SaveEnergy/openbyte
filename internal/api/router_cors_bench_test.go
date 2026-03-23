package api

import "testing"

// BenchmarkRouterIsAllowedOriginExact exercises CORS allow-list matching (exact origin string).
func BenchmarkRouterIsAllowedOriginExact(b *testing.B) {
	r := &Router{allowedOrigins: []string{
		"https://app.example.com",
		"https://partner.example.org",
		"https://admin.internal",
	}}
	origin := "https://app.example.com"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !r.isAllowedOrigin(origin) {
			b.Fatal("expected allowed")
		}
	}
}

// BenchmarkRouterIsAllowedOriginWildcardSubdomain exercises *.domain CORS matching (dot-boundary suffix).
func BenchmarkRouterIsAllowedOriginWildcardSubdomain(b *testing.B) {
	r := &Router{allowedOrigins: []string{"*.cdn.example.org"}}
	origin := "https://static.assets.cdn.example.org"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !r.isAllowedOrigin(origin) {
			b.Fatal("expected allowed")
		}
	}
}
