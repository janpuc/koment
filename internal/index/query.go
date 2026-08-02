package index

import (
	"context"
	"fmt"
	"strings"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/store"
)

// Annotation is a row as the index holds it: annotation content from YAML, plus
// the resolution recorded when the file was last indexed. Callers that serve a
// status must confirm freshness first — see Stale.
type Annotation struct {
	ID         string
	Repository string
	Path       string
	Kind       string
	Scope      string
	Body       string
	Excerpt    string
	Created    string
	AuthorName string
	AuthorKind string
	GitCommit  string
	Status     anchor.Status
	Line       int
}

// FileSummary is one row of the file tree: enough to render a node without
// loading the annotations under it.
type FileSummary struct {
	Path   string
	Count  int
	Worst  anchor.Status
	Counts map[anchor.Status]int
}

// Filter narrows a listing. A zero Filter matches everything.
type Filter struct {
	Repository string
	PathPrefix string
	Kinds      []string
	Statuses   []anchor.Status
	Query      string
	Limit      int
}

func (i *Index) Annotations(ctx context.Context, filter Filter) ([]Annotation, error) {
	where, args := filter.conditions()
	query := `SELECT id, repository_id, path, kind, scope, body, excerpt, created,
	                 author_name, author_kind, git_commit, status, line
	          FROM annotations WHERE ` + strings.Join(where, " AND ") + `
	          ORDER BY path, line, id`
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := i.db.QueryContext(ctx, i.rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("querying annotations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var found []Annotation
	for rows.Next() {
		var a Annotation
		var status string
		if err := rows.Scan(&a.ID, &a.Repository, &a.Path, &a.Kind, &a.Scope, &a.Body,
			&a.Excerpt, &a.Created, &a.AuthorName, &a.AuthorKind, &a.GitCommit, &status, &a.Line); err != nil {
			return nil, fmt.Errorf("scanning annotation: %w", err)
		}
		a.Status = anchor.Status(status)
		found = append(found, a)
	}
	return found, rows.Err()
}

// Files returns the tree, with per-status counts, as one query. Built from a
// directory walk this would be the whole store re-read; that is the reason the
// index exists (ADR 0022).
func (i *Index) Files(ctx context.Context, filter Filter) ([]FileSummary, error) {
	where, args := filter.conditions()
	query := `SELECT path, status, count(*) FROM annotations
	          WHERE ` + strings.Join(where, " AND ") + `
	          GROUP BY path, status ORDER BY path`

	rows, err := i.db.QueryContext(ctx, i.rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("querying files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	order := []string{}
	byPath := map[string]*FileSummary{}
	for rows.Next() {
		var path, status string
		var count int
		if err := rows.Scan(&path, &status, &count); err != nil {
			return nil, fmt.Errorf("scanning file: %w", err)
		}
		summary, seen := byPath[path]
		if !seen {
			summary = &FileSummary{Path: path, Counts: map[anchor.Status]int{}}
			byPath[path] = summary
			order = append(order, path)
		}
		summary.Count += count
		summary.Counts[anchor.Status(status)] = count
		if summary.Worst == "" || severity[anchor.Status(status)] > severity[summary.Worst] {
			summary.Worst = anchor.Status(status)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	summaries := make([]FileSummary, 0, len(order))
	for _, path := range order {
		summaries = append(summaries, *byPath[path])
	}
	return summaries, nil
}

// Search is full text rather than substring: FTS5 on SQLite, a GIN-indexed
// tsvector on Postgres. Both stem, so "annotating" finds "annotation".
func (i *Index) Search(ctx context.Context, filter Filter) ([]Annotation, error) {
	if strings.TrimSpace(filter.Query) == "" {
		return nil, fmt.Errorf("search needs a query")
	}

	if i.driver == Postgres {
		filter.Query = strings.TrimSpace(filter.Query)
		return i.searchPostgres(ctx, filter)
	}
	return i.searchSQLite(ctx, filter)
}

func (i *Index) searchSQLite(ctx context.Context, filter Filter) ([]Annotation, error) {
	where, args := filter.conditionsOn("a.")
	args = append([]any{sanitiseFTS(filter.Query)}, args...)
	query := `SELECT a.id, a.repository_id, a.path, a.kind, a.scope, a.body, a.excerpt, a.created,
	                 a.author_name, a.author_kind, a.git_commit, a.status, a.line
	          FROM annotation_search s
	          JOIN annotations a ON a.id = s.annotation_id AND a.repository_id = s.repository_id
	          WHERE annotation_search MATCH ? AND ` + strings.Join(where, " AND ") + `
	          ORDER BY rank`
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	return i.scanAnnotations(ctx, query, args)
}

func (i *Index) searchPostgres(ctx context.Context, filter Filter) ([]Annotation, error) {
	where, args := filter.conditions()
	// The query text appears twice — once to match, once to rank — so it is
	// bound twice rather than reused, which keeps rebind's numbering honest.
	args = append(args, filter.Query, filter.Query)

	query := `SELECT id, repository_id, path, kind, scope, body, excerpt, created,
	                 author_name, author_kind, git_commit, status, line
	          FROM annotations
	          WHERE ` + strings.Join(where, " AND ") + `
	            AND body_search @@ plainto_tsquery('english', ?)
	          ORDER BY ts_rank(body_search, plainto_tsquery('english', ?)) DESC`
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	return i.scanAnnotations(ctx, query, args)
}

func (i *Index) scanAnnotations(ctx context.Context, query string, args []any) ([]Annotation, error) {
	rows, err := i.db.QueryContext(ctx, i.rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var found []Annotation
	for rows.Next() {
		var a Annotation
		var status string
		if err := rows.Scan(&a.ID, &a.Repository, &a.Path, &a.Kind, &a.Scope, &a.Body,
			&a.Excerpt, &a.Created, &a.AuthorName, &a.AuthorKind, &a.GitCommit, &status, &a.Line); err != nil {
			return nil, fmt.Errorf("scanning match: %w", err)
		}
		a.Status = anchor.Status(status)
		found = append(found, a)
	}
	return found, rows.Err()
}

// Counts is the tally, as one aggregate rather than a walk.
func (i *Index) Counts(ctx context.Context, filter Filter) (map[anchor.Status]int, error) {
	where, args := filter.conditions()
	rows, err := i.db.QueryContext(ctx, i.rebind(
		`SELECT status, count(*) FROM annotations WHERE `+strings.Join(where, " AND ")+` GROUP BY status`), args...)
	if err != nil {
		return nil, fmt.Errorf("counting: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := map[anchor.Status]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[anchor.Status(status)] = count
	}
	return counts, rows.Err()
}

// Refresh re-resolves only the files whose source changed since they were
// indexed, so a served status always matches the working tree without re-reading
// files that did not move (ADR 0022).
func (i *Index) Refresh(ctx context.Context, annotations *store.Store, repository Repository) (int, error) {
	stale, err := i.Stale(ctx, annotations, repository)
	if err != nil {
		return 0, err
	}
	if len(stale) == 0 {
		return 0, nil
	}

	transaction, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning refresh: %w", err)
	}
	defer transaction.Rollback() //nolint:errcheck // only runs if Commit did not

	for _, file := range stale {
		for _, statement := range []string{
			`DELETE FROM annotations WHERE repository_id = ? AND path = ?`,
			`DELETE FROM files WHERE repository_id = ? AND path = ?`,
		} {
			if _, err := transaction.ExecContext(ctx, i.rebind(statement), repository.ID, file); err != nil {
				return 0, fmt.Errorf("clearing %s: %w", file, err)
			}
		}
		if i.driver == SQLite {
			if _, err := transaction.ExecContext(ctx,
				`DELETE FROM annotation_search WHERE repository_id = ? AND annotation_id IN
				 (SELECT id FROM annotations WHERE repository_id = ? AND path = ?)`,
				repository.ID, repository.ID, file); err != nil {
				return 0, fmt.Errorf("clearing search for %s: %w", file, err)
			}
		}
		if err := i.indexFile(ctx, transaction, repository, annotations, file); err != nil {
			return 0, err
		}
	}

	if err := transaction.Commit(); err != nil {
		return 0, fmt.Errorf("committing refresh: %w", err)
	}
	return len(stale), nil
}

// Stale reports the files whose source has changed since it was indexed, so a
// caller re-resolves those and only those. Serving a status without this check
// would show drift that has been fixed, or miss drift that has appeared — the
// exact failure koment exists to prevent (ADR 0022).
func (i *Index) Stale(ctx context.Context, annotations *store.Store, repository Repository) ([]string, error) {
	rows, err := i.db.QueryContext(ctx, i.rebind(
		`SELECT path, mtime_unix, size, present FROM files WHERE repository_id = ?`), repository.ID)
	if err != nil {
		return nil, fmt.Errorf("reading file stamps: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var changed []string
	for rows.Next() {
		var path string
		var indexed stamp
		if err := rows.Scan(&path, &indexed.mtime, &indexed.size, &indexed.present); err != nil {
			return nil, err
		}
		if current := stampOf(annotations, path); current != indexed {
			changed = append(changed, path)
		}
	}
	return changed, rows.Err()
}

func (f Filter) conditions() ([]string, []any) { return f.conditionsOn("") }

// conditionsOn qualifies each column with a table alias, which the search join
// needs because repository_id exists on both sides of it.
func (f Filter) conditionsOn(prefix string) ([]string, []any) {
	where := []string{"1 = 1"}
	var args []any

	if f.Repository != "" {
		where = append(where, prefix+"repository_id = ?")
		args = append(args, f.Repository)
	}
	if f.PathPrefix != "" {
		where = append(where, "("+prefix+"path = ? OR "+prefix+"path LIKE ?)")
		args = append(args, f.PathPrefix, f.PathPrefix+"/%")
	}
	if len(f.Kinds) > 0 {
		where = append(where, prefix+"kind IN ("+placeholders(len(f.Kinds))+")")
		for _, kind := range f.Kinds {
			args = append(args, kind)
		}
	}
	if len(f.Statuses) > 0 {
		where = append(where, prefix+"status IN ("+placeholders(len(f.Statuses))+")")
		for _, status := range f.Statuses {
			args = append(args, string(status))
		}
	}
	return where, args
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// sanitiseFTS keeps a user's words and drops the FTS5 operators, so a query
// containing a quote or a NEAR is a search for those words rather than a syntax
// error thrown at the person typing.
func sanitiseFTS(query string) string {
	var words []string
	for _, field := range strings.Fields(query) {
		cleaned := strings.Map(func(r rune) rune {
			if r == '"' || r == '*' || r == '(' || r == ')' || r == ':' || r == '^' || r == '-' {
				return -1
			}
			return r
		}, field)
		if cleaned != "" {
			words = append(words, `"`+cleaned+`"`)
		}
	}
	return strings.Join(words, " ")
}

var severity = map[anchor.Status]int{
	anchor.StatusOK:       0,
	anchor.StatusMoved:    1,
	anchor.StatusDrifted:  2,
	anchor.StatusOrphaned: 3,
}
