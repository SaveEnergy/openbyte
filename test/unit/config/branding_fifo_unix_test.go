//go:build linux || darwin

package config_test

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestBrandLogoRejectsNamedPipeWithoutBlocking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logo.png")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("create named pipe: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.BrandLogoPath = path
	done := make(chan error, 1)
	go func() {
		done <- cfg.Validate()
	}()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "must be a regular file") {
			t.Fatalf("Validate error = %v, want regular-file error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Validate blocked while opening a named pipe")
	}
}
