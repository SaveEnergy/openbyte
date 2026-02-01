package main

import (
	"fmt"
	"os"
	"strings"

	client "github.com/saveenergy/openbyte/cmd/client"
	server "github.com/saveenergy/openbyte/cmd/server"
)

var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(server.Run(version))
	}

	switch args[0] {
	case "server":
		os.Exit(server.Run(version))
	case "client":
		os.Exit(client.Run(args[1:], version))
	case "help", "-h", "--help":
		printUsage()
		return
	case "version", "--version":
		fmt.Printf("openbyte %s\n", version)
		return
	default:
		if strings.HasPrefix(args[0], "-") {
			os.Exit(server.Run(version))
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

Examples:
  openbyte server
  openbyte client -p tcp -d download -t 30
`)
}
