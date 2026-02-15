package api

import (
	"testing"
	"time"
)

func TestUploadReadDeadline(t *testing.T) {
	start := time.Date(2026, 2, 15, 7, 0, 0, 0, time.UTC)

	if got := uploadReadDeadline(start, 60); !got.Equal(start.Add(60 * time.Second)) {
		t.Fatalf("deadline = %v, want %v", got, start.Add(60*time.Second))
	}

	if got := uploadReadDeadline(start, 0); !got.Equal(start.Add(300 * time.Second)) {
		t.Fatalf("default deadline = %v, want %v", got, start.Add(300*time.Second))
	}
}
