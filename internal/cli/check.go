package cli

import (
	"fmt"

	"github.com/janpuc/koment/internal/anchor"
)

func runCheck(args []string, env Environment) int {
	flags := flagSet("check", env)
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}

	annotations, err := openStore()
	if err != nil {
		return fail(env, err)
	}
	resolved, err := resolveEverything(annotations, flags.Args())
	if err != nil {
		return fail(env, err)
	}

	for _, entry := range resolved {
		writeFailures(env, entry)
	}

	counted := tallyOf(resolved)
	fmt.Fprintf(env.Stdout, "%d annotations across %d files: %s\n", counted.total(), len(resolved), counted)

	if counted.failures() > 0 {
		fmt.Fprintf(env.Stderr, "koment: %d annotations no longer resolve; revisit them or update the anchor\n", counted.failures())
		return ExitFailure
	}
	return ExitOK
}

func writeFailures(env Environment, entry fileResolutions) {
	failing := make([]anchor.Resolution, 0, len(entry.resolutions))
	for _, resolution := range entry.resolutions {
		if resolution.Status.IsFailure() {
			failing = append(failing, resolution)
		}
	}
	if len(failing) == 0 {
		return
	}

	fmt.Fprintf(env.Stdout, "%s\n", entry.file)
	for _, resolution := range failing {
		writeResolution(env.Stdout, entry.file, resolution)
	}
}
