package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/provenance"
	"github.com/janpuc/koment/internal/store"
)

func runAdd(args []string, env Environment) int {
	flags := flagSet("add", env)
	excerpt := flags.String("excerpt", "", "verbatim snippet to anchor to; omit to annotate the whole file")
	kind := flags.String("kind", "", "one of why, gotcha, invariant, anti-pattern")
	title := flags.String("title", "", "short headline shown beside the code; derived from the body when omitted")
	body := flags.String("body", "", "the rationale; - reads it from stdin")
	author := flags.String("author", "", `override the git identity; "Name" or "Name <email>"`)
	byAgent := flags.Bool("agent", false, "record this as written by an agent, not a person")
	target, ok := onePositional("add", "a file", flags, args, env)
	if !ok {
		return ExitUsage
	}

	parsedKind, err := store.ParseType(*kind)
	if err != nil {
		return misuse(env, "%v", err)
	}
	text, err := bodyText(*body, env.Stdin)
	if err != nil {
		return misuse(env, "%v", err)
	}
	service, annotations, err := openApplication()
	if err != nil {
		return fail(env, err)
	}
	createdBy, err := identity(annotations.Root(), *author, *byAgent)
	if err != nil {
		return misuse(env, "%v", err)
	}
	file, err := annotations.FromWorkingDirectory(target)
	if err != nil {
		return fail(env, err)
	}

	mutation, err := service.Add(application.AddInput{
		File: file, Excerpt: *excerpt, Kind: parsedKind, Title: *title, Body: text, Author: createdBy,
	})
	if err != nil {
		return fail(env, err)
	}
	writeMutationWarnings(env, mutation)
	fmt.Fprintf(env.Stdout, "%s  %s %s\n", mutation.Record.Metadata.ID, mutation.Record.Spec.Type, location(file, mutation.Record.Status.LastSeenLine))
	return ExitOK
}

func identity(root, explicit string, byAgent bool) (store.Author, error) {
	kind := store.AuthorHuman
	if byAgent {
		kind = store.AuthorAgent
	}
	if explicit != "" {
		author, err := provenance.ParseAuthor(explicit, kind)
		if err != nil {
			return store.Author{}, err
		}
		return *author, nil
	}

	author, err := provenance.IdentityFromGit(root)
	if err != nil {
		return store.Author{}, err
	}
	author.Kind = kind
	return *author, nil
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

func writeMutationWarnings(env Environment, mutation application.Mutation) {
	for _, warning := range mutation.Warnings {
		fmt.Fprintf(env.Stderr, "koment: %s\n", warning)
	}
}
