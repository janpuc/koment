// Package cli implements the koment commands a human or a shell hook runs.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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
  koment mcp

check exits non-zero when an annotation is drifted or orphaned.
`

type Environment struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// MCPServer runs the MCP server, parsing its own transport flags.
type MCPServer func(args []string, stderr io.Writer) error

// Run dispatches a subcommand.
func Run(args []string, env Environment, serveMCP MCPServer) int {
	if len(args) == 0 {
		fmt.Fprint(env.Stderr, usage)
		return ExitUsage
	}

	command, rest := args[0], args[1:]
	run, known := map[string]func([]string, Environment) int{
		"add":   runAdd,
		"show":  runShow,
		"check": runCheck,
		"list":  runList,
	}[command]

	switch {
	case known:
		return run(rest, env)
	case command == "mcp":
		if err := serveMCP(rest, env.Stderr); err != nil {
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

func oneFileArgument(command string, flags *flag.FlagSet, args []string, env Environment) (string, bool) {
	file, rest := leadingNonFlag(args)
	if err := flags.Parse(rest); err != nil {
		return "", false
	}

	switch {
	case file == "":
		file = flags.Arg(0)
	case flags.NArg() > 0:
		misuse(env, "%s takes one file, also got %s", command, strings.Join(flags.Args(), " "))
		return "", false
	}

	if file == "" {
		misuse(env, "%s needs a file", command)
		return "", false
	}
	return file, true
}

func leadingNonFlag(args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
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
