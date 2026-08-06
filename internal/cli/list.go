package cli

import (
	"fmt"

	"github.com/koment-dev/koment/internal/store"
)

func runList(args []string, env Environment) int {
	flags := flagSet("list", env)
	kind := flags.String("kind", "", "show only this kind")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}

	wanted := store.Type("")
	if *kind != "" {
		parsed, err := store.ParseType(*kind)
		if err != nil {
			return misuse(env, "%v", err)
		}
		wanted = parsed
	}

	service, annotations, err := openApplication()
	if err != nil {
		return fail(env, err)
	}
	resolved, err := resolveEverything(service, annotations, flags.Args())
	if err != nil {
		return fail(env, err)
	}

	shown := tally{}
	for _, entry := range resolved {
		header := false
		for _, resolution := range entry.resolutions {
			if wanted != "" && resolution.Annotation.Spec.Type != wanted {
				continue
			}
			if !header {
				fmt.Fprintf(env.Stdout, "%s\n", entry.file)
				header = true
			}
			writeResolution(env.Stdout, entry.file, resolution)
			shown.add(resolution.Status)
		}
	}

	fmt.Fprintf(env.Stdout, "%d annotations: %s\n", shown.total(), shown)
	if shown.failures() > 0 {
		return ExitFailure
	}
	return ExitOK
}
