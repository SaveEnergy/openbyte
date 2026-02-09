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

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(server.Run(nil, version))
	}

	switch args[0] {
	case "server":
		os.Exit(server.Run(args[1:], version))
	case "client":
		os.Exit(client.Run(args[1:], version))
	case "check":
		os.Exit(check.Run(args[1:], version))
	case "mcp":
		os.Exit(mcpcmd.Run(version))
	case "help", "-h", "--help":
		printUsage()
		return
	case "version", "--version":
		fmt.Printf("openbyte %s\n", version)
		return
	default:
		if strings.HasPrefix(args[0], "-") {
			os.Exit(server.Run(args, version))
		}
		fmt.Fprintf(os.Stderr, "openbyte: unknown command %q\n\n", args[0])
		printUsage()
		os.Exit(2)
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
