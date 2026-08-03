package application

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/repository"
	"github.com/janpuc/koment/internal/store"
)

func TestSnapshotBuildsEveryStatusFromOneRead(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "current.go"), []byte("package sample\n\nvar Current = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	annotations := store.Open(root)
	records := []store.Annotation{
		testRecord("01JQ8ZK3M4N5P6R7S8T9V0W1X2", "current.go", "var Current = true", 3),
		testRecord("01JQ8ZK3M4N5P6R7S8T9V0W1X3", "missing.go", "gone", 1),
	}
	for index := range records {
		if err := annotations.Save(&records[index]); err != nil {
			t.Fatal(err)
		}
	}

	snapshot, err := BuildSnapshot(repository.Repository{ID: "sample", Name: "Sample", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(snapshot.Files))
	}
	counts := snapshot.Counts()
	if counts[anchor.StatusOK] != 1 || counts[anchor.StatusOrphaned] != 1 {
		t.Fatalf("counts = %#v", counts)
	}
	if matches := snapshot.Search("test agent"); len(matches) != 2 {
		t.Fatalf("author search returned %d records", len(matches))
	}
}

func TestServiceAddsAndReanchorsThroughOneContract(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "main.go")
	if err := os.WriteFile(source, []byte("package main\n\nvar First = true\nvar Second = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService(repository.Repository{ID: "sample", Root: root})
	author := store.Author{Name: "Test Agent", Kind: store.AuthorAgent, Source: store.FromExplicit}

	created, err := service.Add(AddInput{
		File: "main.go", Excerpt: "var First = true", Kind: store.KindWhy,
		Body: "The first value is the compatibility default.", Author: author,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Path != ".koment/annotations/"+created.Record.ID+".yaml" {
		t.Fatalf("path = %q", created.Path)
	}

	moved, err := service.Reanchor(ReanchorInput{ID: created.Record.ID, Excerpt: "var Second = true"})
	if err != nil {
		t.Fatal(err)
	}
	if moved.Record.ID != created.Record.ID || moved.Record.Created != created.Record.Created || moved.Record.Author != created.Record.Author {
		t.Fatal("reanchor changed stable record identity")
	}
	if moved.Record.Anchor.Excerpt != "var Second = true" {
		t.Fatalf("excerpt = %q", moved.Record.Anchor.Excerpt)
	}
}

func testRecord(id, file, excerpt string, line int) store.Annotation {
	return store.Annotation{
		Version: store.RecordVersion, ID: id, File: file, Kind: store.KindWhy,
		Body: "The test agent recorded this.", Created: store.Date{Time: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)},
		Anchor: store.Anchor{Scope: store.ScopeExcerpt, Excerpt: excerpt, LastSeenLine: line},
		Author: store.Author{Name: "Test Agent", Kind: store.AuthorAgent, Source: store.FromExplicit},
	}
}
