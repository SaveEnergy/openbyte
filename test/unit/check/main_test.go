package check_test

import (
	"testing"

	"github.com/saveenergy/openbyte/cmd/check"
)

func TestRunUnreachableServerReturnsFailure(t *testing.T) {
	exitCode := check.Run([]string{"--json", "--server-url", "http://127.0.0.1:1", "--timeout", "1"}, "test")
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
}

func TestRunInvalidPositionalArgReturnsUsage(t *testing.T) {
	exitCode := check.Run([]string{"not-a-valid-url"}, "test")
	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
}

func TestRunRejectsExtraPositionalArgs(t *testing.T) {
	exitCode := check.Run([]string{"https://example.com", "https://example.org"}, "test")
	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
}

func TestRunRejectsNonPositiveTimeout(t *testing.T) {
	exitCode := check.Run([]string{"--timeout", "0"}, "test")
	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
}

func TestRunRejectsExcessiveTimeout(t *testing.T) {
	exitCode := check.Run([]string{"--timeout", "301"}, "test")
	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
}
