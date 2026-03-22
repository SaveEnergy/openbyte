package stream_test

import "testing"

const sendBufferSize = 256 * 1024

func BenchmarkDownloadChunkCopy(b *testing.B) {
	randomData := make([]byte, 1024*1024)
	buf := make([]byte, sendBufferSize)
	offset := 0
	dataLen := len(randomData)

	b.ResetTimer()
	for range b.N {
		n := copy(buf, randomData[offset:])
		if n < len(buf) {
			copy(buf[n:], randomData[:len(buf)-n])
		}
		// Equivalent to (offset+len(buf))%dataLen while offset<dataLen and len(buf)<dataLen
		// (avoids integer division on the hot path for these bench sizes).
		offset += len(buf)
		if offset >= dataLen {
			offset -= dataLen
		}
	}
}

func BenchmarkDownloadChunkSlice(b *testing.B) {
	randomData := make([]byte, 1024*1024)
	offset := 0
	dataLen := len(randomData)
	chunkSize := sendBufferSize

	b.ResetTimer()
	for range b.N {
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
