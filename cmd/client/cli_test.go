package client

import (
	"strconv"
	"testing"
)

func TestClientRejectsExtraPositionalArgs(t *testing.T) {
	_, _, code, err := parseFlags([]string{"https://example.com", "https://example.org"}, "test")
	if err == nil {
		t.Fatal("expected error for extra positional args")
	}
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestClientRejectsInvalidServerURLs(t *testing.T) {
	_, _, _, err := parseFlags([]string{"https://example.com?x=1"}, "test")
	if err == nil {
		t.Fatal("expected positional URL with query to be rejected")
	}

	cfg := &ConfigFile{
		ServerURL: "https://example.com#frag",
	}
	if validateConfigFile(cfg) == nil {
		t.Fatal("expected config server_url with fragment to be rejected")
	}
}

func TestClientRejectsAliasPositionals(t *testing.T) {
	_, _, code, err := parseFlags([]string{"nyc"}, "test")
	if err == nil {
		t.Fatal("expected bare alias positional to be rejected")
	}
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestClientRejectsRemovedServerSelectionFlags(t *testing.T) {
	for _, args := range [][]string{{"--servers"}, {"--auto"}, {"-a"}, {"--server", "nyc"}} {
		_, _, code, err := parseFlags(args, "test")
		if err == nil {
			t.Fatalf("expected args %v to be rejected", args)
		}
		if code != exitUsage {
			t.Fatalf("exit code for %v = %d, want %d", args, code, exitUsage)
		}
	}
}

func TestClientRejectsRemovedProtocolFlags(t *testing.T) {
	for _, args := range [][]string{{"--protocol", "tcp"}, {"-p", "udp"}, {"--packet-size", "1400"}} {
		_, _, code, err := parseFlags(args, "test")
		if err == nil {
			t.Fatalf("expected args %v to be rejected", args)
		}
		if code != exitUsage {
			t.Fatalf("exit code for %v = %d, want %d", args, code, exitUsage)
		}
	}
}

func TestClientRejectsBidirectionalDirection(t *testing.T) {
	cfg := &Config{
		ServerURL:  "http://localhost:8080",
		Direction:  "bidirectional",
		Duration:   1,
		Streams:    1,
		ChunkSize:  65536,
		Timeout:    1,
		NoProgress: true,
	}
	if err := validateConfig(cfg); err == nil {
		t.Fatal("expected bidirectional direction to be rejected")
	}
}

func TestClientReducesImplicitWarmUpForShortDurations(t *testing.T) {
	tests := []struct {
		duration int
		want     int
	}{
		{duration: 1, want: 0},
		{duration: 2, want: 1},
		{duration: 3, want: defaultWarmUp},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.duration)+"s", func(t *testing.T) {
			cfg := mergedCLIConfig(t, "--duration", strconv.Itoa(tt.duration))
			if cfg.WarmUp != tt.want {
				t.Fatalf("warm-up = %d, want %d", cfg.WarmUp, tt.want)
			}
			if err := validateConfig(cfg); err != nil {
				t.Fatalf("short duration should remain valid: %v", err)
			}
		})
	}
}

func TestClientValidatesExplicitWarmUpAgainstDuration(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "one second no warm-up", args: []string{"--duration", "1", "--warmup", "0"}},
		{name: "two seconds one second warm-up", args: []string{"--duration", "2", "--warmup", "1"}},
		{name: "negative warm-up", args: []string{"--duration", "2", "--warmup", "-1"}, wantErr: true},
		{name: "warm-up equals duration", args: []string{"--duration", "2", "--warmup", "2"}, wantErr: true},
		{name: "warm-up exceeds duration", args: []string{"--duration", "2", "--warmup", "3"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mergedCLIConfig(t, tt.args...)
			err := validateConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClientRejectsExplicitZeroDuration(t *testing.T) {
	cfg := mergedCLIConfig(t, "--duration", "0")
	if err := validateConfig(cfg); err == nil {
		t.Fatal("expected explicit zero duration to be rejected")
	}
}

func mergedCLIConfig(t *testing.T, args ...string) *Config {
	t.Helper()
	flagConfig, flagsSet, _, err := parseFlags(args, "test")
	if err != nil {
		t.Fatalf("parseFlags(%v): %v", args, err)
	}
	return mergeConfig(flagConfig, nil, flagsSet)
}

func TestConfigFileRejectsRemovedTCPUDPProtocol(t *testing.T) {
	cfg := &ConfigFile{Protocol: "udp"}
	if err := validateConfigFile(cfg); err == nil {
		t.Fatal("expected removed UDP protocol to be rejected")
	}
}

func TestConfigFileAllowsLegacyHTTPProtocol(t *testing.T) {
	cfg := &ConfigFile{Protocol: "http"}
	if err := validateConfigFile(cfg); err != nil {
		t.Fatalf("expected legacy HTTP protocol setting to be accepted: %v", err)
	}
}
