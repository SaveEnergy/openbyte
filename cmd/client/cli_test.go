package client

import "testing"

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
	if err := validateConfigFile(cfg); err == nil {
		t.Fatal("expected config server_url with fragment to be rejected")
	}
}
