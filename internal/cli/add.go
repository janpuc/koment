package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/store"
)

func runAdd(args []string, env Environment) int {
	flags := flagSet("add", env)
	excerpt := flags.String("excerpt", "", "verbatim snippet to anchor to; omit to annotate the whole file")
	kind := flags.String("kind", "", "one of why, gotcha, invariant, anti-pattern")
	body := flags.String("body", "", "the rationale; - reads it from stdin")
	target, ok := onePositional("add", "a file", flags, args, env)
	if !ok {
		return ExitUsage
	}

	annotation, err := buildAnnotation(*kind, *body, env.Stdin)
	if err != nil {
		return misuse(env, "%v", err)
	}

	annotations, err := openStore()
	if err != nil {
		return fail(env, err)
	}
	file, err := annotations.FromWorkingDirectory(target)
	if err != nil {
		return fail(env, err)
	}

	if err := anchorTo(&annotation, annotations, file, *excerpt); err != nil {
		return fail(env, err)
	}

	record, err := appendAnnotation(annotations, file, annotation)
	if err != nil {
		return fail(env, err)
	}
	if err := annotations.Save(record); err != nil {
		return fail(env, err)
	}

	fmt.Fprintf(env.Stdout, "%s  %s %s\n", annotation.ID, annotation.Kind, location(file, annotation.LastSeenLine))
	return ExitOK
}

func buildAnnotation(kind, body string, stdin io.Reader) (store.Annotation, error) {
	parsedKind, err := store.ParseKind(kind)
	if err != nil {
		return store.Annotation{}, err
	}

	text, err := bodyText(body, stdin)
	if err != nil {
		return store.Annotation{}, err
	}

	id, err := store.NewID(time.Now())
	if err != nil {
		return store.Annotation{}, err
	}
	return store.Annotation{ID: id, Kind: parsedKind, Body: store.WrapProse(text), Created: store.Today()}, nil
}

func bodyText(body string, stdin io.Reader) (string, error) {
	if body != "-" {
		if body == "" {
			return "", errors.New("add needs a --body")
		}
		return body, nil
	}

	piped, err := io.ReadAll(stdin)
	if err != nil {
		return "", fmt.Errorf("reading the body from stdin: %w", err)
	}
	if len(piped) == 0 {
		return "", errors.New("--body - was given but stdin was empty")
	}
	return string(piped), nil
}

func anchorTo(annotation *store.Annotation, annotations *store.Store, file, excerpt string) error {
	sourcePath, err := annotations.SourcePath(file)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", file, err)
	}

	if excerpt == "" {
		annotation.Scope = store.ScopeFile
		annotation.Excerpt = ""
		annotation.ExcerptSHA256 = ""
		annotation.LastSeenLine = 0
		return nil
	}

	lines := anchor.ExcerptLines(content, excerpt)
	switch len(lines) {
	case 0:
		return fmt.Errorf("excerpt not found in %s; it must match the file verbatim", file)
	case 1:
		annotation.Scope = store.ScopeExcerpt
		annotation.Excerpt = excerpt
		annotation.ExcerptSHA256 = store.ExcerptSHA256(excerpt)
		annotation.LastSeenLine = lines[0]
		return nil
	default:
		return fmt.Errorf("excerpt matches %d places in %s (lines %v); extend it until it is unique", len(lines), file, lines)
	}
}

func appendAnnotation(annotations *store.Store, file string, annotation store.Annotation) (*store.Record, error) {
	record, err := annotations.Load(file)
	if errors.Is(err, fs.ErrNotExist) {
		return &store.Record{
			Version:     store.RecordVersion,
			File:        file,
			Annotations: []store.Annotation{annotation},
		}, nil
	}
	if err != nil {
		return nil, err
	}

	record.Annotations = append(record.Annotations, annotation)
	return record, nil
}
