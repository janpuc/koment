package cli

import (
	"errors"
	"fmt"
	"io/fs"

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
	record, index, err := annotations.FindByID(id)
	if err != nil {
		return fail(env, err)
	}

	target := record.File
	if *destination != "" {
		if target, err = annotations.FromWorkingDirectory(*destination); err != nil {
			return fail(env, err)
		}
	}

	moved := record.Annotations[index]
	if err := reanchorTo(&moved, annotations, target, *excerpt); err != nil {
		return fail(env, err)
	}

	if err := relocate(annotations, record, index, target, moved); err != nil {
		return fail(env, err)
	}

	fmt.Fprintf(env.Stdout, "%s  %s %s\n", moved.ID, moved.Kind, location(target, moved.LastSeenLine))
	return ExitOK
}

// reanchorTo keeps the annotation's existing excerpt when only the file
// changed, so a renamed file does not also require retyping the snippet.
func reanchorTo(annotation *store.Annotation, annotations *store.Store, file, excerpt string) error {
	if excerpt == "" && annotation.Scope == store.ScopeExcerpt {
		excerpt = annotation.Excerpt
	}
	return anchorTo(annotation, annotations, file, excerpt)
}

func relocate(annotations *store.Store, from *store.Record, index int, target string, moved store.Annotation) error {
	if target == from.File {
		from.Annotations[index] = moved
		return annotations.Save(from)
	}

	destination, err := recordFor(annotations, target)
	if err != nil {
		return err
	}
	destination.Annotations = append(destination.Annotations, moved)
	if err := annotations.Save(destination); err != nil {
		return err
	}

	from.Annotations = append(from.Annotations[:index], from.Annotations[index+1:]...)
	if len(from.Annotations) == 0 {
		return annotations.Remove(from.File)
	}
	return annotations.Save(from)
}

func recordFor(annotations *store.Store, file string) (*store.Record, error) {
	record, err := annotations.Load(file)
	if errors.Is(err, fs.ErrNotExist) {
		return &store.Record{Version: store.RecordVersion, File: file}, nil
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}
