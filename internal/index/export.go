package index

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/janpuc/koment/internal/store"
)

// Bootstrap fills an empty index from the YAML in git. A cold cache, a wiped
// volume and a fresh container are the same case, and none of them should need
// a person to notice (ADR 0023).
func (i *Index) Bootstrap(ctx context.Context, repository Repository, annotations *store.Store) (bool, error) {
	var rows int
	if err := i.db.QueryRowContext(ctx, i.rebind(
		`SELECT count(*) FROM annotations WHERE repository_id = ?`), repository.ID).Scan(&rows); err != nil {
		return false, fmt.Errorf("counting the index: %w", err)
	}
	if rows > 0 {
		return false, nil
	}
	return true, i.Rebuild(ctx, repository, annotations)
}

// Export writes the whole .koment store back out from the index. It is the
// inverse of Rebuild, and the two are exact: a record read in and written out
// again is byte-identical (ADR 0023).
//
// It writes through store.Save rather than emitting YAML itself, so there is one
// writer, one validation path, and no second formatter to drift.
func (i *Index) Export(ctx context.Context, repository Repository, into *store.Store) (int, error) {
	annotations, err := i.records(ctx, repository)
	if err != nil {
		return 0, err
	}

	files := make([]string, 0, len(annotations))
	for file := range annotations {
		files = append(files, file)
	}
	sort.Strings(files)

	for _, file := range files {
		record := &store.Record{
			Version:     store.RecordVersion,
			File:        file,
			Annotations: annotations[file],
		}
		if err := into.Save(record); err != nil {
			return 0, fmt.Errorf("writing %s: %w", file, err)
		}
	}
	return len(files), nil
}

// records reconstructs annotations grouped by file. Ordering within a file
// follows the id, so an export is deterministic rather than however the
// database felt like returning rows.
func (i *Index) records(ctx context.Context, repository Repository) (map[string][]store.Annotation, error) {
	rows, err := i.db.QueryContext(ctx, i.rebind(
		`SELECT path, id, scope, excerpt, excerpt_sha256, last_seen_line, kind, body, created,
		        git_commit, git_path, git_line, git_end_line,
		        author_name, author_email, author_kind, author_source, author_account, author_verified
		 FROM annotations WHERE repository_id = ? ORDER BY path, id`), repository.ID)
	if err != nil {
		return nil, fmt.Errorf("reading annotations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	byFile := map[string][]store.Annotation{}
	for rows.Next() {
		var path, created string
		var annotation store.Annotation
		var git store.GitContext
		var author store.Author

		if err := rows.Scan(&path, &annotation.ID, &annotation.Scope, &annotation.Excerpt,
			&annotation.ExcerptSHA256, &annotation.LastSeenLine, &annotation.Kind, &annotation.Body, &created,
			&git.Commit, &git.Path, &git.Line, &git.EndLine,
			&author.Name, &author.Email, &author.Kind, &author.Source, &author.Account, &author.Verified); err != nil {
			return nil, fmt.Errorf("scanning annotation: %w", err)
		}

		parsed, err := time.Parse("2006-01-02", created)
		if err != nil {
			return nil, fmt.Errorf("annotation %s has an unparseable created date %q: %w", annotation.ID, created, err)
		}
		annotation.Created = store.Date{Time: parsed}

		// An absent block stays absent. Writing an empty git or author key
		// would invent provenance the annotation never had.
		if git.Commit != "" {
			annotation.Git = &git
		}
		if author.Name != "" {
			annotation.Author = &author
		}

		byFile[path] = append(byFile[path], annotation)
	}
	return byFile, rows.Err()
}
