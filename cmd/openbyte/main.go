package main

import (
	"fmt"
	"os"
	"strings"

	check "github.com/saveenergy/openbyte/cmd/check"
	client "github.com/saveenergy/openbyte/cmd/client"
	mcpcmd "github.com/saveenergy/openbyte/cmd/mcp"
	server "github.com/saveenergy/openbyte/cmd/server"
)

var version = "dev"

var (
	runServer = func(args []string, ver string) int { return server.Run(args, ver) }
	runClient = func(args []string, ver string) int { return client.Run(args, ver) }
	runCheck  = func(args []string, ver string) int { return check.Run(args, ver) }
	runMCP    = func(args []string, ver string) int { return mcpcmd.Run(ver) }
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
	case "client":
		return runClient(args[1:], ver)
	case "check":
		return runCheck(args[1:], ver)
	case "mcp":
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "openbyte: mcp does not accept arguments: %q\n", strings.Join(args[1:], " "))
			return 2
		}
		return runMCP(args[1:], ver)
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
  client    Run the client CLI
  check     Quick connectivity check (~3-5 seconds)
  mcp       Run as MCP server (stdio transport, for AI agents)

Examples:
  openbyte server
  openbyte client -p tcp -d download -t 30
  openbyte check --json https://speed.example.com
  openbyte mcp
`)
}
