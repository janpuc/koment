package main

import (
	"os"

	"github.com/janpuc/koment/internal/cli"
	"github.com/janpuc/koment/internal/mcp"
	"github.com/janpuc/koment/internal/ui"
)

func main() {
	env := cli.Environment{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	os.Exit(cli.Run(os.Args[1:], env, mcp.Serve, ui.Serve, ui.Export))
}
