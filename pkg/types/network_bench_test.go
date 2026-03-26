package types

import "testing"

// BenchmarkOriginHost is used by CORS and host normalization (URL parse + strip port).
func BenchmarkOriginHost(b *testing.B) {
	const origin = "https://app.example.com:8443/some/path"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = OriginHost(origin)
	}
}

// BenchmarkStripHostPort covers bracketed IPv6 and host:port inputs.
func BenchmarkStripHostPort(b *testing.B) {
	const host = "[2001:db8::1]:443"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = StripHostPort(host)
	}
}

// BenchmarkNetworkInfoSetClientIP exercises sanitize + IPv6 flag (colon probe).
func BenchmarkNetworkInfoSetClientIP(b *testing.B) {
	const addr = "[2001:db8::1]:443"
	var n NetworkInfo

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		n.SetClientIP(addr)
	}
}
