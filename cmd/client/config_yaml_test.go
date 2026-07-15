package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigFile(t *testing.T) {
	writeConfigFile(t, `
server_url: https://speedtest.example.com
protocol: http
direction: download
duration: 30
streams: 8
chunk_size: 1048576
timeout: 45
json: true
no_progress: true
`)

	config, err := loadConfigFile()
	if err != nil {
		t.Fatalf("load config file: %v", err)
	}
	if config == nil {
		t.Fatal("expected config")
	}
	if config.ServerURL != "https://speedtest.example.com" {
		t.Fatalf("server URL = %q, want %q", config.ServerURL, "https://speedtest.example.com")
	}
	if config.Duration != 30 || config.Streams != 8 || config.ChunkSize != 1048576 {
		t.Fatalf("numeric config = duration %d, streams %d, chunk size %d", config.Duration, config.Streams, config.ChunkSize)
	}
	if !config.JSON || !config.NoProgress {
		t.Fatalf("boolean config = JSON %t, no progress %t", config.JSON, config.NoProgress)
	}
}

func TestLoadConfigFileRejectsUnknownField(t *testing.T) {
	writeConfigFile(t, "server_url: https://speedtest.example.com\nstreamz: 8\n")

	_, err := loadConfigFile()
	if err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
	if !strings.Contains(err.Error(), "parse config file:") || !strings.Contains(err.Error(), "field streamz not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigFileRejectsOversizedFile(t *testing.T) {
	writeConfigFile(t, strings.Repeat(" ", maxConfigFileSize+1))

	_, err := loadConfigFile()
	if err == nil {
		t.Fatal("expected oversized config to be rejected")
	}
	want := "read config file: exceeds 65536-byte limit"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestLoadConfigFileRejectsMultipleDocuments(t *testing.T) {
	writeConfigFile(t, "server_url: https://speedtest.example.com\n---\nstreams: 8\n")

	_, err := loadConfigFile()
	if err == nil {
		t.Fatal("expected multiple documents to be rejected")
	}
	want := "parse config file: multiple YAML documents are not allowed"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestLoadConfigFileWrapsValidationError(t *testing.T) {
	writeConfigFile(t, "duration: 301\n")

	_, err := loadConfigFile()
	if err == nil {
		t.Fatal("expected invalid duration to be rejected")
	}
	want := "invalid config file: invalid duration: 301 (must be 1-300 seconds)"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestLoadConfigFileMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	config, err := loadConfigFile()
	if err != nil {
		t.Fatalf("load missing config file: %v", err)
	}
	if config != nil {
		t.Fatalf("config = %#v, want nil", config)
	}
}

func TestLoadConfigFileAllowsEmptyFile(t *testing.T) {
	writeConfigFile(t, "# intentionally empty\n")

	config, err := loadConfigFile()
	if err != nil {
		t.Fatalf("load empty config file: %v", err)
	}
	if config == nil {
		t.Fatal("expected empty config")
	}
}

func writeConfigFile(t *testing.T, contents string) {
	t.Helper()

	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	configPath := filepath.Join(configDir, "openbyte", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
}
