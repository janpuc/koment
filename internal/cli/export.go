package cli

import (
	"context"
	"fmt"

	"github.com/janpuc/koment/internal/index"
)

// runExport rebuilds .koment from the index. It is the inverse of the rebuild
// that fills the index from .koment, and the two are exact (ADR 0023).
func runExport(args []string, env Environment) int {
	flags := flagSet("export", env)
	databaseURL := flags.String("database-url", "", "Postgres connection string; SQLite is used when empty")
	path := flags.String("index", "", "SQLite index file; defaults to a per-repository file in the cache directory")
	name := flags.String("name", "", "repository name recorded in the index; defaults to the directory name")
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

	// An empty index would export an empty store, quietly deleting nothing and
	// achieving nothing. Say so rather than reporting success.
	counts, err := built.Counts(ctx, index.Filter{Repository: repository.ID})
	if err != nil {
		return fail(env, err)
	}
	total := 0
	for _, count := range counts {
		total += count
	}
	if total == 0 {
		return fail(env, fmt.Errorf("the index holds no annotations for %s; run koment index first", repository.Name))
	}

	files, err := built.Export(ctx, repository, annotations)
	if err != nil {
		return fail(env, err)
	}

	fmt.Fprintf(env.Stdout, "wrote %d annotations across %d files to %s\n",
		total, files, annotations.Root()+"/.koment/annotations")
	return ExitOK
}
