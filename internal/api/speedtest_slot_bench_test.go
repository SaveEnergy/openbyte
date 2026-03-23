package api

import "testing"

// BenchmarkSpeedtestSlotAcquireReleaseDownload exercises global + per-IP slot accounting for GET /download.
func BenchmarkSpeedtestSlotAcquireReleaseDownload(b *testing.B) {
	h := NewSpeedTestHandler(1_000_000, 300)
	h.SetMaxConcurrentPerIP(0)
	ip := "192.0.2.77"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !h.tryAcquireSpeedtestSlot(ip, true) {
			b.Fatal("expected acquire")
		}
		h.releaseSpeedtestSlot(ip, true)
	}
}

// BenchmarkSpeedtestSlotAcquireReleaseUpload matches POST /upload slot path.
func BenchmarkSpeedtestSlotAcquireReleaseUpload(b *testing.B) {
	h := NewSpeedTestHandler(1_000_000, 300)
	h.SetMaxConcurrentPerIP(0)
	ip := "192.0.2.78"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !h.tryAcquireSpeedtestSlot(ip, false) {
			b.Fatal("expected acquire")
		}
		h.releaseSpeedtestSlot(ip, false)
	}
}

// BenchmarkResolveRandomSourceShared is the fast path when 4MiB server random buffer is wired (typical prod).
func BenchmarkResolveRandomSourceShared(b *testing.B) {
	h := NewSpeedTestHandler(10, 300)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		src, release, err := h.resolveRandomSource()
		if err != nil {
			b.Fatal(err)
		}
		if len(src) == 0 {
			b.Fatal("empty source")
		}
		release()
	}
}
