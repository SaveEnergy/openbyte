package api

import (
	"bytes"
	"testing"
	"time"
)

func TestResolveRandomSourceFallback(t *testing.T) {
	handler := NewSpeedTestHandler(2, 60)
	handler.randomData = nil

	src, release, err := handler.resolveRandomSource()
	if err != nil {
		t.Fatalf("resolveRandomSource: %v", err)
	}
	defer release()

	if len(src) != 64*1024 {
		t.Errorf("len(src) = %d, want %d", len(src), 64*1024)
	}
	if bytes.Equal(src, make([]byte, 64*1024)) {
		t.Error("expected non-zero random data")
	}
}

func TestUploadReadDeadline(t *testing.T) {
	start := time.Date(2026, 2, 15, 7, 0, 0, 0, time.UTC)

	if got := uploadReadDeadline(start, 60); !got.Equal(start.Add(60 * time.Second)) {
		t.Fatalf("deadline = %v, want %v", got, start.Add(60*time.Second))
	}

	if got := uploadReadDeadline(start, 0); !got.Equal(start.Add(300 * time.Second)) {
		t.Fatalf("default deadline = %v, want %v", got, start.Add(300*time.Second))
	}
}
