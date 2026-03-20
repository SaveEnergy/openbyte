// Package mcp implements the `openbyte mcp` subcommand — an MCP (Model Context
// Protocol) server over stdio transport. Agents can spawn this process and call
// connectivity tools directly.
package mcp

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

// Run starts the MCP stdio server. Blocks until stdin closes or signal received.
func Run(version string) int {
	s := server.NewMCPServer(
		"openbyte",
		version,
		server.WithToolCapabilities(true),
	)
	registerTools(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "openbyte mcp: error: %v\n", err)
		return 1
	}
	return 0
}
