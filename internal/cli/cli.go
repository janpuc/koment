// Package cli implements the koment commands a human or a shell hook runs.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/janpuc/koment/internal/config"
	"github.com/janpuc/koment/internal/store"
)

const (
	ExitOK      = 0
	ExitFailure = 1
	ExitUsage   = 2
)

const usage = `koment — out-of-band code annotations

  koment add <file> [--excerpt <text>] --kind <kind> --body <text|->
  koment show <file>
  koment check [path...]
  koment list [--kind <kind>]
  koment reanchor <id> [--excerpt <text>] [--file <path>]
  koment ui [--listen <addr>]
  koment export [--out <dir>]        rebuild .koment from the index
  koment site --out <dir>            render one repository to static HTML
  koment mcp
  koment version

check exits non-zero when an annotation is drifted or orphaned. reanchor is how
you fix one: it recomputes the hash and the line, and keeps the id.
`

type Environment struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Build  Build
}

// Server runs a long-lived server, parsing its own flags.
type Server func(args []string, stderr io.Writer) error

// Servers are injected rather than imported. Adding one is a new field, not a
// new parameter, so signatures here stop changing shape.
type Servers struct {
	MCP  Server
	UI   Server
	Site Server
}

// Run dispatches a subcommand.
func Run(args []string, env Environment, servers Servers) int {
	if len(args) == 0 {
		fmt.Fprint(env.Stderr, usage)
		return ExitUsage
	}

	command, rest := args[0], args[1:]
	run, known := map[string]func([]string, Environment) int{
		"add":      runAdd,
		"show":     runShow,
		"check":    runCheck,
		"list":     runList,
		"reanchor": runReanchor,
		"index":    runIndex,
		"export":   runExport,
		"version":  runVersion,
	}[command]

	switch {
	case known:
		return run(rest, env)
	case command == "mcp":
		if err := servers.MCP(rest, env.Stderr); err != nil {
			return fail(env, err)
		}
		return ExitOK
	case command == "ui":
		if err := servers.UI(rest, env.Stderr); err != nil {
			return fail(env, err)
		}
		return ExitOK
	case command == "site":
		if err := servers.Site(rest, env.Stderr); err != nil {
			return fail(env, err)
		}
		return ExitOK
	case command == "help", command == "-h", command == "--help":
		fmt.Fprint(env.Stdout, usage)
		return ExitOK
	}

	fmt.Fprintf(env.Stderr, "koment: unknown command %q\n\n%s", command, usage)
	return ExitUsage
}

func fail(env Environment, err error) int {
	fmt.Fprintf(env.Stderr, "koment: %v\n", err)
	return ExitFailure
}

func misuse(env Environment, format string, args ...any) int {
	fmt.Fprintf(env.Stderr, "koment: "+format+"\n", args...)
	return ExitUsage
}

func flagSet(name string, env Environment) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(env.Stderr)
	return flags
}

func onePositional(command, what string, flags *flag.FlagSet, args []string, env Environment) (string, bool) {
	value, rest := leadingNonFlag(args)
	if err := flags.Parse(rest); err != nil {
		return "", false
	}

	switch {
	case value == "":
		value = flags.Arg(0)
	case flags.NArg() > 0:
		misuse(env, "%s takes one %s, also got %s", command, what, strings.Join(flags.Args(), " "))
		return "", false
	}

	if value == "" {
		misuse(env, "%s needs %s", command, what)
		return "", false
	}
	return value, true
}

func leadingNonFlag(args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}

func environmentDefaults(flags *flag.FlagSet) error {
	return config.FromEnvironment(flags)
}

func repositoryName(given, root string) string {
	if given != "" {
		return given
	}
	return filepath.Base(root)
}

func openStore() (*store.Store, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("finding the working directory: %w", err)
	}
	root, err := store.FindRoot(workingDirectory)
	if err != nil {
		return nil, err
	}
	return store.Open(root), nil
}
