package index

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/store"
)

// Repository is a repository as the index knows it. The id is derived from the
// root path so a single-repository checkout needs no configuration; ADR 0017
// wants an assigned id once there is a registry to assign one.
type Repository struct {
	ID   string
	Name string
	Root string
}

func RepositoryFor(annotations *store.Store, name string) Repository {
	root := annotations.Root()
	sum := sha256.Sum256([]byte(root))
	return Repository{
		ID:   hex.EncodeToString(sum[:])[:16],
		Name: name,
		Root: root,
	}
}

// Rebuild replaces a repository's rows from the YAML on disk. It is the only
// way annotations enter the index: nothing writes here directly, so the index
// can always be thrown away and reconstructed (ADR 0022).
func (i *Index) Rebuild(ctx context.Context, repository Repository, annotations *store.Store) error {
	files, err := annotations.AnnotatedFiles()
	if err != nil {
		return err
	}

	transaction, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning rebuild: %w", err)
	}
	defer transaction.Rollback() //nolint:errcheck // rolled back only if Commit did not run

	for _, statement := range []string{
		`DELETE FROM annotations WHERE repository_id = ?`,
		`DELETE FROM files WHERE repository_id = ?`,
	} {
		if _, err := transaction.ExecContext(ctx, i.rebind(statement), repository.ID); err != nil {
			return fmt.Errorf("clearing the index: %w", err)
		}
	}
	if i.driver == SQLite {
		if _, err := transaction.ExecContext(ctx,
			`DELETE FROM annotation_search WHERE repository_id = ?`, repository.ID); err != nil {
			return fmt.Errorf("clearing the search index: %w", err)
		}
	}

	if _, err := transaction.ExecContext(ctx, i.rebind(
		`INSERT INTO repositories (id, name, root, indexed_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET name = excluded.name, root = excluded.root, indexed_at = excluded.indexed_at`),
		repository.ID, repository.Name, repository.Root, time.Now().Unix()); err != nil {
		return fmt.Errorf("recording the repository: %w", err)
	}

	for _, file := range files {
		if err := i.indexFile(ctx, transaction, repository, annotations, file); err != nil {
			return err
		}
	}

	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("committing rebuild: %w", err)
	}
	return nil
}

func (i *Index) indexFile(ctx context.Context, transaction execer, repository Repository, annotations *store.Store, file string) error {
	record, err := annotations.Load(file)
	if err != nil {
		return fmt.Errorf("loading %s: %w", file, err)
	}
	resolutions, err := anchor.ResolveStored(annotations, file)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", file, err)
	}

	stamp := stampOf(annotations, file)
	if _, err := transaction.ExecContext(ctx, i.rebind(
		`INSERT INTO files (repository_id, path, mtime_unix, size, present) VALUES (?, ?, ?, ?, ?)`),
		repository.ID, file, stamp.mtime, stamp.size, stamp.present); err != nil {
		return fmt.Errorf("recording %s: %w", file, err)
	}

	byID := map[string]store.Annotation{}
	for _, annotation := range record.Annotations {
		byID[annotation.ID] = annotation
	}

	for _, resolution := range resolutions {
		annotation := byID[resolution.Annotation.ID]
		if err := i.insertAnnotation(ctx, transaction, repository, file, annotation, resolution); err != nil {
			return err
		}
	}
	return nil
}

func (i *Index) insertAnnotation(ctx context.Context, transaction execer, repository Repository, file string,
	annotation store.Annotation, resolution anchor.Resolution) error {

	authorName, authorKind := "", ""
	if annotation.Author != nil {
		authorName, authorKind = annotation.Author.Name, string(annotation.Author.Kind)
	}
	gitCommit := ""
	if annotation.Git != nil {
		gitCommit = annotation.Git.Commit
	}

	if _, err := transaction.ExecContext(ctx, i.rebind(
		`INSERT INTO annotations
		   (id, repository_id, path, kind, scope, body, excerpt, created,
		    author_name, author_kind, git_commit, status, line)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		annotation.ID, repository.ID, file, string(annotation.Kind), string(annotation.Scope),
		annotation.Body, annotation.Excerpt, annotation.Created.Format("2006-01-02"),
		authorName, authorKind, gitCommit, string(resolution.Status), resolution.Line); err != nil {
		return fmt.Errorf("indexing annotation %s: %w", annotation.ID, err)
	}

	if i.driver == SQLite {
		if _, err := transaction.ExecContext(ctx,
			`INSERT INTO annotation_search (body, annotation_id, repository_id) VALUES (?, ?, ?)`,
			annotation.Body, annotation.ID, repository.ID); err != nil {
			return fmt.Errorf("indexing annotation %s for search: %w", annotation.ID, err)
		}
	}
	return nil
}

// stamp is how the index knows a source file has not changed since it was
// resolved. mtime alone is too coarse — a same-second edit of the same length
// would go unnoticed — so size travels with it (ADR 0022).
type stamp struct {
	mtime   int64
	size    int64
	present bool
}

func stampOf(annotations *store.Store, file string) stamp {
	path, err := annotations.SourcePath(file)
	if err != nil {
		return stamp{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return stamp{}
	}
	return stamp{mtime: info.ModTime().Unix(), size: info.Size(), present: true}
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
