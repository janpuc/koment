package main

import (
	"os"

	"github.com/koment-dev/koment/internal/cli"
	"github.com/koment-dev/koment/internal/lsp"
	"github.com/koment-dev/koment/internal/mcp"
	"github.com/koment-dev/koment/internal/server"
	"github.com/koment-dev/koment/internal/ui"
)

var (
	releaseVersion string
	sourceRevision string
)

func main() {
	env := cli.Environment{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Build:  cli.Build{Version: releaseVersion, Revision: sourceRevision},
	}
	os.Exit(cli.Run(os.Args[1:], env, cli.Servers{
		MCP: mcp.Serve, UI: ui.Serve, Site: ui.Site, Serve: server.Serve, LSP: lsp.Serve,
	}))
}
