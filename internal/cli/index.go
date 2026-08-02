package cli

import (
	"context"
	"fmt"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/index"
)

func runIndex(args []string, env Environment) int {
	flags := flagSet("index", env)
	databaseURL := flags.String("database-url", "", "Postgres connection string; SQLite is used when empty")
	path := flags.String("index", "", "SQLite index file; defaults to a per-repository file in the cache directory")
	name := flags.String("name", "", "repository name recorded in the index; defaults to the directory name")
	rebuild := flags.Bool("rebuild", false, "discard and rebuild rather than refreshing changed files only")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if err := environmentDefaults(flags); err != nil {
		return misuse(env, "%v", err)
	}

	annotations, err := openStore()
	if err != nil {
		return fail(env, err)
	}
	repository := index.RepositoryFor(annotations, repositoryName(*name, annotations.Root()))

	location := *path
	if location == "" && *databaseURL == "" {
		if location, err = index.DefaultPath(repository.ID); err != nil {
			return fail(env, err)
		}
	}

	ctx := context.Background()
	built, err := index.Open(ctx, *databaseURL, location)
	if err != nil {
		return fail(env, err)
	}
	defer func() { _ = built.Close() }()

	if *rebuild {
		if err := built.Rebuild(ctx, repository, annotations); err != nil {
			return fail(env, err)
		}
		fmt.Fprintf(env.Stdout, "rebuilt %s from %s\n", repository.Name, annotations.Root())
	} else {
		touched, err := built.Refresh(ctx, annotations, repository)
		if err != nil {
			return fail(env, err)
		}
		if touched == 0 {
			if err := built.Rebuild(ctx, repository, annotations); err != nil {
				return fail(env, err)
			}
		}
		fmt.Fprintf(env.Stdout, "refreshed %s (%d files re-resolved)\n", repository.Name, touched)
	}

	counts, err := built.Counts(ctx, index.Filter{Repository: repository.ID})
	if err != nil {
		return fail(env, err)
	}
	files, err := built.Files(ctx, index.Filter{Repository: repository.ID})
	if err != nil {
		return fail(env, err)
	}

	total := 0
	for _, status := range []anchor.Status{anchor.StatusOK, anchor.StatusMoved, anchor.StatusDrifted, anchor.StatusOrphaned} {
		total += counts[status]
	}
	fmt.Fprintf(env.Stdout, "%d annotations across %d files, via %s\n", total, len(files), built.Driver())
	return ExitOK
}
