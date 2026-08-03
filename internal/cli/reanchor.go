package cli

import (
	"fmt"

	"github.com/janpuc/koment/internal/store"
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

	annotations, err := openStore()
	if err != nil {
		return fail(env, err)
	}
	record, err := annotations.FindByID(id)
	if err != nil {
		return fail(env, err)
	}

	target := record.File
	if *destination != "" {
		if target, err = annotations.FromWorkingDirectory(*destination); err != nil {
			return fail(env, err)
		}
	}

	moved := *record
	if err := reanchorTo(&moved, annotations, target, *excerpt); err != nil {
		return fail(env, err)
	}
	moved.File = target
	if err := annotations.Save(&moved); err != nil {
		return fail(env, err)
	}

	fmt.Fprintf(env.Stdout, "%s  %s %s\n", moved.ID, moved.Kind, location(target, moved.Anchor.LastSeenLine))
	return ExitOK
}

func reanchorTo(annotation *store.Annotation, annotations *store.Store, file, excerpt string) error {
	if excerpt == "" && annotation.Anchor.Scope == store.ScopeExcerpt {
		excerpt = annotation.Anchor.Excerpt
	}
	return anchorTo(annotation, annotations, file, excerpt)
}
