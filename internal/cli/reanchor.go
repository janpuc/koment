package cli

import (
	"fmt"

	"github.com/janpuc/koment/internal/application"
)

func runReanchor(args []string, env Environment) int {
	flags := flagSet("reanchor", env)
	excerpt := flags.String("excerpt", "", "verbatim snippet to anchor to in the target file")
	destination := flags.String("file", "", "move the annotation to this file")

	id, ok := onePositional("reanchor", "an annotation id", flags, args, env)
	if !ok {
		return ExitUsage
	}
	if *excerpt == "" && *destination == "" {
		return misuse(env, "reanchor needs --excerpt, --file, or both")
	}

	service, annotations, err := openApplication()
	if err != nil {
		return fail(env, err)
	}
	target := ""
	if *destination != "" {
		if target, err = annotations.FromWorkingDirectory(*destination); err != nil {
			return fail(env, err)
		}
	}
	mutation, err := service.Reanchor(application.ReanchorInput{ID: id, File: target, Excerpt: *excerpt})
	if err != nil {
		return fail(env, err)
	}
	fmt.Fprintf(env.Stdout, "%s  %s %s\n", mutation.Record.ID, mutation.Record.Kind, location(mutation.Record.File, mutation.Record.Anchor.LastSeenLine))
	return ExitOK
}
