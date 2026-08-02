package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/store"
)

const source = "package main\n\nfunc main() {\n\tserve()\n}\n"

func annotated(t *testing.T) (*store.Store, Repository) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, store.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	annotations := store.Open(root)
	excerpt := "\tserve()"
	record := &store.Record{
		Version: store.RecordVersion,
		File:    "main.go",
		Annotations: []store.Annotation{
			{
				ID: "01AAAAAAAAAAAAAAAAAAAAAAAA", Scope: store.ScopeExcerpt,
				Excerpt: excerpt, ExcerptSHA256: store.ExcerptSHA256(excerpt), LastSeenLine: 4,
				Kind:    store.KindInvariant,
				Body:    "serve must be the last call: it blocks until the process is signalled.",
				Created: store.Date{Time: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)},
				Author:  &store.Author{Name: "Fixture", Kind: store.AuthorHuman, Source: store.FromGitConfig},
			},
			{
				ID: "01BBBBBBBBBBBBBBBBBBBBBBBB", Scope: store.ScopeFile,
				Kind:    store.KindWhy,
				Body:    "Entry point only; the annotating logic lives in internal.",
				Created: store.Date{Time: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)},
			},
		},
	}
	if err := annotations.Save(record); err != nil {
		t.Fatal(err)
	}
	return annotations, RepositoryFor(annotations, "fixture")
}

func openIndex(t *testing.T) *Index {
	t.Helper()
	i, err := Open(context.Background(), "", filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { i.Close() })
	return i
}

func TestRebuildPopulatesFromYAML(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)

	if err := i.Rebuild(context.Background(), repository, annotations); err != nil {
		t.Fatal(err)
	}

	found, err := i.Annotations(context.Background(), Filter{Repository: repository.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("want 2 annotations, got %d", len(found))
	}
	byID := map[string]Annotation{}
	for _, a := range found {
		byID[a.ID] = a
		if a.Status != anchor.StatusOK {
			t.Errorf("want resolution recorded for %s, got %q", a.ID, a.Status)
		}
	}
	if got := byID["01AAAAAAAAAAAAAAAAAAAAAAAA"].AuthorName; got != "Fixture" {
		t.Errorf("author did not reach the index: %q", got)
	}
	if got := byID["01BBBBBBBBBBBBBBBBBBBBBBBB"].AuthorName; got != "" {
		t.Errorf("an annotation with no author must not gain one: %q", got)
	}
}

func TestRebuildIsIdempotent(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()

	for range 3 {
		if err := i.Rebuild(ctx, repository, annotations); err != nil {
			t.Fatal(err)
		}
	}

	found, err := i.Annotations(ctx, Filter{Repository: repository.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Errorf("rebuilding must replace rather than accumulate, got %d annotations", len(found))
	}
}

func TestSearchIsFullTextAndStems(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	matches, err := i.Search(ctx, Filter{Repository: repository.ID, Query: "signalled"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("want 1 match, got %d", len(matches))
	}

	// "annotating" in the body should be found by "annotation" — a substring
	// scan cannot do this, which is the point of using FTS.
	stemmed, err := i.Search(ctx, Filter{Repository: repository.ID, Query: "annotation"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stemmed) != 1 {
		t.Errorf("want the stemmer to match annotating/annotation, got %d", len(stemmed))
	}

	none, err := i.Search(ctx, Filter{Repository: repository.ID, Query: "nothing here at all"})
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("want no matches, got %d", len(none))
	}
}

func TestSearchSurvivesOperatorCharacters(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	// A person typing a quote or a bare operator should get a search, not a
	// syntax error thrown back at them.
	for _, query := range []string{`"serve`, `serve*`, `NEAR(serve`, `serve OR`, `(serve)`} {
		if _, err := i.Search(ctx, Filter{Repository: repository.ID, Query: query}); err != nil {
			t.Errorf("query %q should not error: %v", query, err)
		}
	}
}

func TestFilesReturnsTheTreeWithCounts(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	files, err := i.Files(ctx, Filter{Repository: repository.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "main.go" {
		t.Fatalf("want one file main.go, got %+v", files)
	}
	if files[0].Count != 2 {
		t.Errorf("want 2 annotations on the file, got %d", files[0].Count)
	}
	if files[0].Worst != anchor.StatusOK {
		t.Errorf("want worst status ok, got %q", files[0].Worst)
	}
}

func TestFilterNarrowsByKindStatusAndPath(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	byKind, err := i.Annotations(ctx, Filter{Repository: repository.ID, Kinds: []string{"why"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(byKind) != 1 || byKind[0].Kind != "why" {
		t.Errorf("kind filter did not narrow: %+v", byKind)
	}

	byStatus, err := i.Annotations(ctx, Filter{Repository: repository.ID, Statuses: []anchor.Status{anchor.StatusDrifted}})
	if err != nil {
		t.Fatal(err)
	}
	if len(byStatus) != 0 {
		t.Errorf("nothing is drifted, got %d", len(byStatus))
	}

	byPath, err := i.Annotations(ctx, Filter{Repository: repository.ID, PathPrefix: "nowhere"})
	if err != nil {
		t.Fatal(err)
	}
	if len(byPath) != 0 {
		t.Errorf("path filter did not narrow, got %d", len(byPath))
	}
}

func TestCountsAggregate(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	counts, err := i.Counts(ctx, Filter{Repository: repository.ID})
	if err != nil {
		t.Fatal(err)
	}
	if counts[anchor.StatusOK] != 2 {
		t.Errorf("want 2 ok, got %v", counts)
	}
}

func TestStaleDetectsAnEditedSourceFile(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	stale, err := i.Stale(ctx, annotations, repository)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 0 {
		t.Fatalf("nothing changed, want no stale files, got %v", stale)
	}

	// A same-second edit that also changes length: mtime alone might miss it,
	// which is why size travels with the stamp.
	edited := source + "\n// appended\n"
	if err := os.WriteFile(filepath.Join(annotations.Root(), "main.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	stale, err = i.Stale(ctx, annotations, repository)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 || stale[0] != "main.go" {
		t.Errorf("an edited file must be reported stale, got %v", stale)
	}
}

func TestStaleDetectsADeletedSourceFile(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(annotations.Root(), "main.go")); err != nil {
		t.Fatal(err)
	}
	stale, err := i.Stale(ctx, annotations, repository)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 {
		t.Errorf("a deleted file must be reported stale, got %v", stale)
	}
}

func TestRebuildPicksUpDrift(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	edited := "package main\n\nfunc main() {\n\tserveForever()\n}\n"
	if err := os.WriteFile(filepath.Join(annotations.Root(), "main.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	drifted, err := i.Annotations(ctx, Filter{Repository: repository.ID, Statuses: []anchor.Status{anchor.StatusDrifted}})
	if err != nil {
		t.Fatal(err)
	}
	if len(drifted) != 1 {
		t.Errorf("want the drifted annotation, got %d", len(drifted))
	}
}

func TestRepositoryIDIsStableForTheSameRoot(t *testing.T) {
	annotations, first := annotated(t)
	second := RepositoryFor(annotations, "renamed")

	if first.ID != second.ID {
		t.Errorf("the id must follow the root, not the display name: %s vs %s", first.ID, second.ID)
	}
	if second.Name != "renamed" {
		t.Errorf("the name should follow what was given, got %q", second.Name)
	}
}

func TestReopeningKeepsTheData(t *testing.T) {
	annotations, repository := annotated(t)
	path := filepath.Join(t.TempDir(), "index.db")
	ctx := context.Background()

	first, err := Open(ctx, "", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}
	first.Close()

	second, err := Open(ctx, "", path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	found, err := second.Annotations(ctx, Filter{Repository: repository.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Errorf("a warm start should find the previous index, got %d", len(found))
	}
}

func TestDefaultPathIsOutsideTheRepository(t *testing.T) {
	path, err := DefaultPath("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("want an absolute cache path, got %q", path)
	}
	if filepath.Ext(path) != ".db" {
		t.Errorf("want a .db file, got %q", path)
	}
}

func TestRefreshOnlyTouchesChangedFilesAndFixesTheStatus(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	touched, err := i.Refresh(ctx, annotations, repository)
	if err != nil {
		t.Fatal(err)
	}
	if touched != 0 {
		t.Errorf("nothing changed, so nothing should be re-resolved; got %d", touched)
	}

	edited := "package main\n\nfunc main() {\n\tserveForever()\n}\n"
	if err := os.WriteFile(filepath.Join(annotations.Root(), "main.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	touched, err = i.Refresh(ctx, annotations, repository)
	if err != nil {
		t.Fatal(err)
	}
	if touched != 1 {
		t.Errorf("want the one changed file re-resolved, got %d", touched)
	}

	drifted, err := i.Annotations(ctx, Filter{Repository: repository.ID, Statuses: []anchor.Status{anchor.StatusDrifted}})
	if err != nil {
		t.Fatal(err)
	}
	if len(drifted) != 1 {
		t.Errorf("refresh must correct the served status, got %d drifted", len(drifted))
	}
}

// TestPostgresBehavesLikeSQLite runs only when a database is provided. CI
// supplies one as a service container; locally it is skipped, so a claim about
// Postgres is never made on the strength of an untested code path.
func TestPostgresBehavesLikeSQLite(t *testing.T) {
	url := os.Getenv("KOMENT_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set KOMENT_TEST_DATABASE_URL to exercise the Postgres driver")
	}

	annotations, repository := annotated(t)
	ctx := context.Background()

	i, err := Open(ctx, url, "")
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()

	if i.Driver() != Postgres {
		t.Fatalf("want the postgres driver, got %s", i.Driver())
	}
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	found, err := i.Annotations(ctx, Filter{Repository: repository.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("want 2 annotations, got %d", len(found))
	}

	matches, err := i.Search(ctx, Filter{Repository: repository.ID, Query: "signalled"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Errorf("postgres full-text search should match, got %d", len(matches))
	}

	files, err := i.Files(ctx, Filter{Repository: repository.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Count != 2 || files[0].Worst != anchor.StatusOK {
		t.Errorf("file tree differs from sqlite: %+v", files)
	}

	counts, err := i.Counts(ctx, Filter{Repository: repository.ID})
	if err != nil {
		t.Fatal(err)
	}
	if counts[anchor.StatusOK] != 2 {
		t.Errorf("want 2 ok, got %v", counts)
	}
}

// TestFilterValuesAreBoundNotInterpolated is the enforcement behind the gosec
// exclusion for this package: a hostile filter value must be treated as data,
// never spliced into the SQL.
func TestFilterValuesAreBoundNotInterpolated(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	hostile := "'; DROP TABLE annotations; --"
	for _, filter := range []Filter{
		{Repository: hostile},
		{Repository: repository.ID, PathPrefix: hostile},
		{Repository: repository.ID, Kinds: []string{hostile}},
		{Repository: repository.ID, Statuses: []anchor.Status{anchor.Status(hostile)}},
		{Repository: repository.ID, Query: hostile},
	} {
		if filter.Query != "" {
			if _, err := i.Search(ctx, filter); err != nil {
				t.Errorf("hostile query should be data, not an error: %v", err)
			}
			continue
		}
		if _, err := i.Annotations(ctx, filter); err != nil {
			t.Errorf("hostile filter should be data, not an error: %v", err)
		}
	}

	// The table is still here, which is the whole point.
	survived, err := i.Annotations(ctx, Filter{Repository: repository.ID})
	if err != nil {
		t.Fatalf("annotations table did not survive: %v", err)
	}
	if len(survived) != 2 {
		t.Errorf("want 2 annotations still present, got %d", len(survived))
	}
}
