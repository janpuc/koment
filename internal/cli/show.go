package cli

import (
	"fmt"

	"github.com/koment-dev/koment/internal/anchor"
)

func runShow(args []string, env Environment) int {
	flags := flagSet("show", env)
	target, ok := onePositional("show", "a file", flags, args, env)
	if !ok {
		return ExitUsage
	}

	service, annotations, err := openApplication()
	if err != nil {
		return fail(env, err)
	}
	file, err := annotations.FromWorkingDirectory(target)
	if err != nil {
		return fail(env, err)
	}

	snapshot, err := service.Snapshot()
	if err != nil {
		return fail(env, err)
	}
	fileSnapshot, found := snapshot.File(file)
	resolutions := make([]anchor.Resolution, 0, len(fileSnapshot.Annotations))
	if found {
		for _, view := range fileSnapshot.Annotations {
			resolutions = append(resolutions, anchor.Resolution{
				Annotation: view.Record, Status: view.Status, Line: view.Line, Occurrences: view.Occurrences,
			})
		}
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
