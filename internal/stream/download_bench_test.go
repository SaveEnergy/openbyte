package stream

import (
	"testing"
)

func BenchmarkDownloadChunkCopy(b *testing.B) {
	randomData := make([]byte, 1024*1024)
	buf := make([]byte, sendBufferSize)
	offset := 0
	dataLen := len(randomData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := copy(buf, randomData[offset:])
		if n < len(buf) {
			copy(buf[n:], randomData[:len(buf)-n])
		}
		offset = (offset + len(buf)) % dataLen
	}
}

func BenchmarkDownloadChunkSlice(b *testing.B) {
	randomData := make([]byte, 1024*1024)
	offset := 0
	dataLen := len(randomData)
	chunkSize := sendBufferSize

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if offset+chunkSize <= dataLen {
			_ = randomData[offset : offset+chunkSize]
			offset += chunkSize
			if offset == dataLen {
				offset = 0
			}
			continue
		}
		first := randomData[offset:]
		_ = first
		remaining := chunkSize - len(first)
		if remaining > 0 {
			_ = randomData[:remaining]
		}
		offset = remaining
		if offset >= dataLen {
			offset = 0
		}
	}
}
