package cli

import (
	"fmt"

	"github.com/janpuc/koment/internal/anchor"
)

func runShow(args []string, env Environment) int {
	flags := flagSet("show", env)
	target, ok := onePositional("show", "a file", flags, args, env)
	if !ok {
		return ExitUsage
	}

	annotations, err := openStore()
	if err != nil {
		return fail(env, err)
	}
	file, err := annotations.FromWorkingDirectory(target)
	if err != nil {
		return fail(env, err)
	}

	resolutions, err := anchor.ResolveStored(annotations, file)
	if err != nil {
		return fail(env, err)
	}
	if len(resolutions) == 0 {
		fmt.Fprintf(env.Stdout, "%s has no annotations\n", file)
		return ExitOK
	}

	fmt.Fprintf(env.Stdout, "%s\n", file)
	for _, resolution := range resolutions {
		writeResolution(env.Stdout, file, resolution)
	}

	if countFailures(resolutions) > 0 {
		return ExitFailure
	}
	return ExitOK
}

func countFailures(resolutions []anchor.Resolution) int {
	failures := 0
	for _, resolution := range resolutions {
		if resolution.Status.IsFailure() {
			failures++
		}
	}
	return failures
}
