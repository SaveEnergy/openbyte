package main

import "testing"

func TestRunDispatch(t *testing.T) {
	oldServer, oldClient, oldCheck, oldMCP := runServer, runClient, runCheck, runMCP
	t.Cleanup(func() {
		runServer, runClient, runCheck, runMCP = oldServer, oldClient, oldCheck, oldMCP
	})

	var got struct {
		target string
		args   []string
	}

	runServer = func(args []string, _ string) int {
		got.target = "server"
		got.args = append([]string(nil), args...)
		return 11
	}
	runClient = func(args []string, _ string) int {
		got.target = "client"
		got.args = append([]string(nil), args...)
		return 12
	}
	runCheck = func(args []string, _ string) int {
		got.target = "check"
		got.args = append([]string(nil), args...)
		return 13
	}
	runMCP = func(_ string) int {
		got.target = "mcp"
		got.args = nil
		return 14
	}

	tests := []struct {
		name       string
		args       []string
		wantTarget string
		wantExit   int
	}{
		{name: "default server", args: nil, wantTarget: "server", wantExit: 11},
		{name: "server subcommand", args: []string{"server", "--x"}, wantTarget: "server", wantExit: 11},
		{name: "client subcommand", args: []string{"client", "--y"}, wantTarget: "client", wantExit: 12},
		{name: "check subcommand", args: []string{"check", "--json"}, wantTarget: "check", wantExit: 13},
		{name: "mcp subcommand", args: []string{"mcp"}, wantTarget: "mcp", wantExit: 14},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got.target = ""
			got.args = nil
			code := run(tc.args, "test")
			if code != tc.wantExit {
				t.Fatalf("exit code = %d, want %d", code, tc.wantExit)
			}
			if got.target != tc.wantTarget {
				t.Fatalf("target = %q, want %q", got.target, tc.wantTarget)
			}
		})
	}
}

func TestRunHelpVersionAndUnknown(t *testing.T) {
	if code := run([]string{"help"}, "test"); code != 0 {
		t.Fatalf("help exit code = %d, want 0", code)
	}
	if code := run([]string{"--help"}, "test"); code != 0 {
		t.Fatalf("--help exit code = %d, want 0", code)
	}
	if code := run([]string{"version"}, "test"); code != 0 {
		t.Fatalf("version exit code = %d, want 0", code)
	}
	if code := run([]string{"unknown-cmd"}, "test"); code != 2 {
		t.Fatalf("unknown exit code = %d, want 2", code)
	}
	if code := run([]string{"--unknown-flag"}, "test"); code != 2 {
		t.Fatalf("unknown top-level flag exit code = %d, want 2", code)
	}
}
