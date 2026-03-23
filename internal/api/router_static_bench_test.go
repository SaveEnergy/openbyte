package api

import "testing"

// benchStaticAllowlist mirrors production keys used by isAllowedStaticAsset (subset sufficient for map + fonts rule).
var benchStaticAllowlist = map[string]bool{
	"index.html":  true,
	"openbyte.js": true,
	"base.css":    true,
}

// BenchmarkIsAllowedStaticAssetAllowlisted is the fast path for top-level JS/CSS/HTML names.
func BenchmarkIsAllowedStaticAssetAllowlisted(b *testing.B) {
	const name = "openbyte.js"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !isAllowedStaticAsset(name, benchStaticAllowlist) {
			b.Fatal("expected allowed")
		}
	}
}

// BenchmarkIsAllowedStaticAssetFont exercises fonts/*.woff2 allow path (suffix checks).
func BenchmarkIsAllowedStaticAssetFont(b *testing.B) {
	const name = "fonts/inter-latin.woff2"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !isAllowedStaticAsset(name, benchStaticAllowlist) {
			b.Fatal("expected allowed")
		}
	}
}

// BenchmarkIsAllowedStaticAssetReject is a cheap rejection (no map hit, no fonts prefix).
func BenchmarkIsAllowedStaticAssetReject(b *testing.B) {
	const name = "../../../etc/passwd"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if isAllowedStaticAsset(name, benchStaticAllowlist) {
			b.Fatal("expected reject")
		}
	}
}
