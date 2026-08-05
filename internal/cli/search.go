package cli

import (
	"fmt"

	"github.com/janpuc/koment/internal/anchor"
)

func runSearch(args []string, env Environment) int {
	flags := flagSet("search", env)
	query, ok := onePositional("search", "a query", flags, args, env)
	if !ok {
		return ExitUsage
	}

	service, _, err := openApplication()
	if err != nil {
		return fail(env, err)
	}
	snapshot, err := service.Snapshot()
	if err != nil {
		return fail(env, err)
	}

	matches := snapshot.Search(query)
	for _, match := range matches {
		writeResolution(env.Stdout, match.Record.Spec.Target.File, anchor.Resolution{
			Annotation: match.Record, Status: match.Status, Line: match.Line, Occurrences: match.Occurrences,
		})
	}
	fmt.Fprintf(env.Stdout, "%d annotations matched %q\n", len(matches), query)

	for _, match := range matches {
		if match.Status.IsFailure() {
			return ExitFailure
		}
	}
	return ExitOK
}
