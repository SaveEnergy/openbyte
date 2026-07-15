package main

import (
	"fmt"
	"os"
	"strings"

	check "github.com/saveenergy/openbyte/cmd/check"
	server "github.com/saveenergy/openbyte/cmd/server"
)

var version = "dev"

var (
	runServer = func(args []string, ver string) int { return server.Run(args, ver) }
	runCheck  = func(args []string, ver string) int { return check.Run(args, ver) }
)

func main() {
	os.Exit(run(os.Args[1:], version))
}

func run(args []string, ver string) int {
	if len(args) == 0 {
		return runServer(nil, ver)
	}

	switch args[0] {
	case "server":
		return runServer(args[1:], ver)
	case "check":
		return runCheck(args[1:], ver)
	case "help", "-h", "--help":
		printUsage()
		return 0
	case "version", "--version":
		fmt.Printf("openbyte %s\n", ver)
		return 0
	default:
		if strings.HasPrefix(args[0], "-") {
			fmt.Fprintf(os.Stderr, "openbyte: unknown top-level flag %q\n\n", args[0])
			printUsage()
			return 2
		}
		fmt.Fprintf(os.Stderr, "openbyte: unknown command %q\n\n", args[0])
		printUsage()
		return 2
	}
}

func printUsage() {
	fmt.Fprintf(os.Stdout, `Usage: openbyte <command> [args]

Commands:
  server    Run the speed test server (default when no command provided)
  check     Quick connectivity check (~3-5 seconds)

Examples:
  openbyte server
  openbyte check --json https://speed.example.com
`)
}
