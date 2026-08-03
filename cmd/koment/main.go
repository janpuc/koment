package main

import (
	"os"

	"github.com/janpuc/koment/internal/cli"
	"github.com/janpuc/koment/internal/mcp"
	"github.com/janpuc/koment/internal/ui"
)

// Stamped by the release build. Left empty everywhere else, where the module
// version Go records in the binary is the better answer anyway.
var (
	version  string
	revision string
)

func main() {
	env := cli.Environment{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Build:  cli.Build{Version: version, Revision: revision},
	}
	os.Exit(cli.Run(os.Args[1:], env, cli.Servers{MCP: mcp.Serve, UI: ui.Serve, Site: ui.Site}))
}
