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
