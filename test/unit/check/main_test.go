package check_test

import (
	"testing"

	"github.com/saveenergy/openbyte/cmd/check"
)

const (
	testCommandName  = "test"
	exitCodeFailure  = 1
	exitCodeUsage    = 2
	unreachableURL   = "http://127.0.0.1:1"
	invalidURLArg    = "not-a-valid-url"
	timeoutFlag      = "--timeout"
	timeoutZero      = "0"
	timeoutExcessive = "301"
	exitCodeWantFmt  = "exit code = %d, want %d"
)

func TestRunUnreachableServerReturnsFailure(t *testing.T) {
	exitCode := check.Run([]string{"--json", "--server-url", unreachableURL, timeoutFlag, "1"}, testCommandName)
	if exitCode != exitCodeFailure {
		t.Fatalf(exitCodeWantFmt, exitCode, exitCodeFailure)
	}
}

func TestRunInvalidPositionalArgReturnsUsage(t *testing.T) {
	exitCode := check.Run([]string{invalidURLArg}, testCommandName)
	if exitCode != exitCodeUsage {
		t.Fatalf(exitCodeWantFmt, exitCode, exitCodeUsage)
	}
}

func TestRunRejectsExtraPositionalArgs(t *testing.T) {
	exitCode := check.Run([]string{"https://example.com", "https://example.org"}, testCommandName)
	if exitCode != exitCodeUsage {
		t.Fatalf(exitCodeWantFmt, exitCode, exitCodeUsage)
	}
}

func TestRunRejectsNonPositiveTimeout(t *testing.T) {
	exitCode := check.Run([]string{timeoutFlag, timeoutZero}, testCommandName)
	if exitCode != exitCodeUsage {
		t.Fatalf(exitCodeWantFmt, exitCode, exitCodeUsage)
	}
}

func TestRunRejectsExcessiveTimeout(t *testing.T) {
	exitCode := check.Run([]string{timeoutFlag, timeoutExcessive}, testCommandName)
	if exitCode != exitCodeUsage {
		t.Fatalf(exitCodeWantFmt, exitCode, exitCodeUsage)
	}
}
