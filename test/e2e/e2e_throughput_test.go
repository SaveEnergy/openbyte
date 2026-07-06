package e2e

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Loopback throughput floors are deliberately conservative: they exist to
// catch catastrophic hot-path regressions (accidental sleeps, tiny buffers,
// per-chunk syscall storms), not to benchmark. Loopback normally sustains
// tens of Gbit/s; 100 Mbit/s stays safe on shared CI runners under -race.
const (
	throughputStreams   = 4
	throughputSeconds   = 2
	throughputFloorMbps = 100.0
	uploadPayloadBytes  = 16 * 1024 * 1024
)

func throughputMbps(totalBytes int64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	return float64(totalBytes*8) / elapsed.Seconds() / 1e6
}

func TestLoopbackDownloadThroughputFloor(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	var total int64
	var wg sync.WaitGroup
	start := time.Now()
	for range throughputStreams {
		wg.Add(1)
		go func() {
			defer wg.Done()
			url := fmt.Sprintf("%s/api/v1/download?duration=%d&chunk=1048576",
				ts.baseURL, throughputSeconds)
			resp, err := http.Get(url)
			if err != nil {
				t.Errorf("download request failed: %v", err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("download status = %d, want %d", resp.StatusCode, http.StatusOK)
				return
			}
			n, err := io.Copy(io.Discard, resp.Body)
			if err != nil {
				t.Errorf("download read failed: %v", err)
				return
			}
			atomic.AddInt64(&total, n)
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	got := throughputMbps(atomic.LoadInt64(&total), elapsed)
	t.Logf("loopback download: %.0f Mbps (%d bytes over %v, %d streams)",
		got, atomic.LoadInt64(&total), elapsed.Round(time.Millisecond), throughputStreams)
	if got < throughputFloorMbps {
		t.Fatalf("download throughput = %.1f Mbps, want >= %.0f Mbps", got, throughputFloorMbps)
	}
}

func TestLoopbackUploadThroughputFloor(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	payload := make([]byte, uploadPayloadBytes)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("generate upload payload: %v", err)
	}

	deadline := time.Now().Add(throughputSeconds * time.Second)
	var total int64
	var wg sync.WaitGroup
	start := time.Now()
	for range throughputStreams {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				resp, err := http.Post(ts.baseURL+"/api/v1/upload",
					"application/octet-stream", bytes.NewReader(payload))
				if err != nil {
					t.Errorf("upload request failed: %v", err)
					return
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Errorf("upload status = %d, want %d", resp.StatusCode, http.StatusOK)
					return
				}
				atomic.AddInt64(&total, uploadPayloadBytes)
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	got := throughputMbps(atomic.LoadInt64(&total), elapsed)
	t.Logf("loopback upload: %.0f Mbps (%d bytes over %v, %d streams)",
		got, atomic.LoadInt64(&total), elapsed.Round(time.Millisecond), throughputStreams)
	if got < throughputFloorMbps {
		t.Fatalf("upload throughput = %.1f Mbps, want >= %.0f Mbps", got, throughputFloorMbps)
	}
}
