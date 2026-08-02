// Package index holds a queryable copy of the annotations that live in git.
// It is derived: never edited directly, never the only copy of anything, and a
// missing or stale index is a rebuild rather than a loss (ADR 0022).
package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Driver string

const (
	SQLite   Driver = "sqlite"
	Postgres Driver = "postgres"
)

type Index struct {
	db     *sql.DB
	driver Driver
}

// Open connects to the index. A DATABASE_URL selects Postgres, which is what
// makes a multi-replica deployment stateless; otherwise a local SQLite file,
// which needs no configuration at all.
func Open(ctx context.Context, databaseURL, sqlitePath string) (*Index, error) {
	if databaseURL != "" {
		return openPostgres(ctx, databaseURL)
	}
	return openSQLite(ctx, sqlitePath)
}

func openSQLite(ctx context.Context, path string) (*Index, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}

	// WAL keeps a reader from blocking the rebuild; busy_timeout stops a
	// concurrent rebuild failing outright rather than waiting.
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}

	i := &Index{db: db, driver: SQLite}
	if err := i.migrate(ctx, sqliteSchema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return i, nil
}

func openPostgres(ctx context.Context, url string) (*Index, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("opening postgres: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}

	i := &Index{db: db, driver: Postgres}
	if err := i.migrate(ctx, postgresSchema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return i, nil
}

func (i *Index) Close() error { return i.db.Close() }

func (i *Index) Driver() Driver { return i.driver }

// migrate discards an index built by an older schema rather than migrating it.
// The index is derived, so throwing it away costs a rebuild and nothing else —
// and a rebuild is always correct, where a migration can be subtly wrong.
func (i *Index) migrate(ctx context.Context, statements []string) error {
	for _, statement := range statements {
		if _, err := i.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("applying schema: %w", err)
		}
	}

	var stored string
	err := i.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&stored)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return i.setSchemaVersion(ctx)
	case err != nil:
		return fmt.Errorf("reading schema version: %w", err)
	case stored != fmt.Sprint(schemaVersion):
		if err := i.discard(ctx); err != nil {
			return err
		}
		return i.setSchemaVersion(ctx)
	}
	return nil
}

func (i *Index) setSchemaVersion(ctx context.Context) error {
	_, err := i.db.ExecContext(ctx,
		i.rebind(`INSERT INTO meta (key, value) VALUES ('schema_version', ?)
		          ON CONFLICT (key) DO UPDATE SET value = excluded.value`),
		fmt.Sprint(schemaVersion))
	if err != nil {
		return fmt.Errorf("recording schema version: %w", err)
	}
	return nil
}

func (i *Index) discard(ctx context.Context) error {
	for _, table := range []string{"annotations", "files", "repositories"} {
		if _, err := i.db.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("discarding %s: %w", table, err)
		}
	}
	if i.driver == SQLite {
		if _, err := i.db.ExecContext(ctx, "DELETE FROM annotation_search"); err != nil {
			return fmt.Errorf("discarding annotation_search: %w", err)
		}
	}
	return nil
}

// rebind turns ? placeholders into $1, $2 … for Postgres, so every query in
// this package can be written once in the more readable form.
func (i *Index) rebind(query string) string {
	if i.driver != Postgres {
		return query
	}

	var out strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			fmt.Fprintf(&out, "$%d", n)
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// DefaultPath is where a SQLite index lives: the user's cache directory, keyed
// by repository. It is a build artifact, not repository content, and is never
// committed (ADR 0022).
func DefaultPath(repositoryID string) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("finding the cache directory: %w", err)
	}
	return filepath.Join(cache, "koment", repositoryID+".db"), nil
}
