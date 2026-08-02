package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/janpuc/koment/internal/store"
)

// TestRoundTripIsByteIdentical is the property ADR 0023 rests on: reading
// .koment into the index and writing it back out must lose nothing. It uses a
// record exercising every optional field, so a field added to Annotation and
// forgotten in the schema fails here rather than silently vanishing later.
func TestRoundTripIsByteIdentical(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, store.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	annotations := store.Open(root)
	excerpt := "\tserve()"
	original := &store.Record{
		Version: store.RecordVersion,
		File:    "main.go",
		Annotations: []store.Annotation{
			{
				ID: "01AAAAAAAAAAAAAAAAAAAAAAAA", Scope: store.ScopeExcerpt,
				Excerpt: excerpt, ExcerptSHA256: store.ExcerptSHA256(excerpt), LastSeenLine: 4,
				Kind: store.KindGotcha, Body: store.WrapProse("Every optional field is set here on purpose."),
				Created: store.Date{Time: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)},
				Git: &store.GitContext{
					Commit: "9f3c1a4d8e2b7c5a6f0d3e1b8c7a5f2d4e6b9c1a",
					Path:   "old/path/main.go", Line: 12, EndLine: 18,
				},
				Author: &store.Author{
					Name: "Jan Pucilowski", Email: "janpuc@proton.me",
					Kind: store.AuthorHuman, Source: store.FromSession,
					Account: "gh:janpuc", Verified: "oidc",
				},
			},
			{
				ID: "01BBBBBBBBBBBBBBBBBBBBBBBB", Scope: store.ScopeFile,
				Kind: store.KindWhy, Body: "No git block, no author block.",
				Created: store.Date{Time: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)},
			},
		},
	}
	if err := annotations.Save(original); err != nil {
		t.Fatal(err)
	}

	recordPath, err := annotations.RecordPath("main.go")
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	repository := RepositoryFor(annotations, "fixture")
	i, err := Open(ctx, "", filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	// Destroy the store entirely, then rebuild it from the index alone.
	if err := os.RemoveAll(filepath.Join(root, store.DirName, "annotations")); err != nil {
		t.Fatal(err)
	}
	written, err := i.Export(ctx, repository, annotations)
	if err != nil {
		t.Fatal(err)
	}
	if written != 1 {
		t.Fatalf("want 1 file written, got %d", written)
	}

	after, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("export did not recreate the record: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("round trip is not byte-identical\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

func TestBootstrapFillsAnEmptyIndexAndIsNotRepeated(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()

	filled, err := i.Bootstrap(ctx, repository, annotations)
	if err != nil {
		t.Fatal(err)
	}
	if !filled {
		t.Error("an empty index should have been bootstrapped")
	}

	found, err := i.Annotations(ctx, Filter{Repository: repository.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("want 2 annotations after bootstrap, got %d", len(found))
	}

	again, err := i.Bootstrap(ctx, repository, annotations)
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Error("a populated index must not be rebuilt on open; that would discard interactive state")
	}
}

func TestExportRecreatesAStoreLostEntirely(t *testing.T) {
	annotations, repository := annotated(t)
	i := openIndex(t)
	ctx := context.Background()
	if err := i.Rebuild(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(filepath.Join(annotations.Root(), store.DirName, "annotations")); err != nil {
		t.Fatal(err)
	}
	if _, err := annotations.AnnotatedFiles(); err != nil {
		t.Fatal(err)
	}

	if _, err := i.Export(ctx, repository, annotations); err != nil {
		t.Fatal(err)
	}
	files, err := annotations.AnnotatedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("want the store recreated, got %v", files)
	}

	record, err := annotations.Load("main.go")
	if err != nil {
		t.Fatalf("recreated record does not load: %v", err)
	}
	if len(record.Annotations) != 2 {
		t.Errorf("want 2 annotations recovered, got %d", len(record.Annotations))
	}
}
