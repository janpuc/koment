package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koment-dev/koment/internal/anchor"
	"github.com/koment-dev/koment/internal/repository"
	"github.com/koment-dev/koment/internal/store"
)

func TestAssembleSnapshotResolvesProviderContentWithoutFilesystemAccess(t *testing.T) {
	record := testRecord("01JQ8ZK3M4N5P6R7S8T9V0W1X2", "remote.go", "var Remote = true", 3)
	generated := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	snapshot, err := AssembleSnapshot(SnapshotInput{
		Repository:  RepositoryIdentity{ID: "remote", Name: "Remote"},
		Commit:      strings.Repeat("a", 40),
		GeneratedAt: generated,
		Records:     []store.Annotation{record},
		Sources:     map[string][]byte{"remote.go": []byte("package sample\n\nvar Remote = true\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Commit != strings.Repeat("a", 40) || !snapshot.GeneratedAt.Equal(generated) {
		t.Fatalf("snapshot metadata = %#v", snapshot)
	}
	if len(snapshot.Files) != 1 || snapshot.Files[0].Annotations[0].Status != anchor.StatusOK {
		t.Fatalf("files = %#v", snapshot.Files)
	}
}

func TestAssembleSnapshotRejectsDuplicateRecordIdentity(t *testing.T) {
	record := testRecord("01JQ8ZK3M4N5P6R7S8T9V0W1X2", "remote.go", "var Remote = true", 3)
	_, err := AssembleSnapshot(SnapshotInput{Records: []store.Annotation{record, record}})
	if err == nil || !strings.Contains(err.Error(), "duplicate annotation id") {
		t.Fatalf("err = %v", err)
	}
}

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
		File: "main.go", Excerpt: "var First = true", Kind: store.TypeWhy,
		Body: "The first value is the compatibility default.", Author: author,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Path != ".koment/annotations/"+created.Record.Metadata.ID+".yaml" {
		t.Fatalf("path = %q", created.Path)
	}
	hasHeadlineWarning(t, created.Warnings, true)

	explicit, err := service.Add(AddInput{
		File: "main.go", Excerpt: "var Second = true", Kind: store.TypeWhy,
		Title: "Why the first value wins",
		Body:  "The first value is the compatibility default.", Author: author,
	})
	if err != nil {
		t.Fatal(err)
	}
	if explicit.Record.Spec.Title != "Why the first value wins" {
		t.Fatalf("title = %q", explicit.Record.Spec.Title)
	}
	hasHeadlineWarning(t, explicit.Warnings, false)

	moved, err := service.Reanchor(ReanchorInput{ID: created.Record.Metadata.ID, Excerpt: "var Second = true"})
	if err != nil {
		t.Fatal(err)
	}
	if moved.Record.Metadata.ID != created.Record.Metadata.ID || moved.Record.Metadata.Created != created.Record.Metadata.Created || moved.Record.Spec.Author != created.Record.Spec.Author {
		t.Fatal("reanchor changed stable record identity")
	}
	if moved.Record.Spec.Anchor.Excerpt != "var Second = true" {
		t.Fatalf("excerpt = %q", moved.Record.Spec.Anchor.Excerpt)
	}
}

func TestAddWithoutATitleWarnsThatTheHeadlineIsDerived(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nvar First = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService(repository.Repository{ID: "sample", Root: root})
	author := store.Author{Name: "Test Agent", Kind: store.AuthorAgent, Source: store.FromExplicit}

	created, err := service.Add(AddInput{
		File: "main.go", Excerpt: "var First = true", Kind: store.TypeWhy,
		Body: "The first value is the compatibility default.", Author: author,
	})
	if err != nil {
		t.Fatal(err)
	}
	hasHeadlineWarning(t, created.Warnings, true)
}

func hasHeadlineWarning(t *testing.T, warnings []string, want bool) {
	t.Helper()
	for _, warning := range warnings {
		if strings.Contains(warning, "first sentence of the body will be shown") {
			if want {
				return
			}
			t.Fatalf("unexpected headline warning: %q", warning)
		}
	}
	if want {
		t.Fatalf("expected a headline warning, got %v", warnings)
	}
}

func testRecord(id, file, excerpt string, line int) store.Annotation {
	return store.Annotation{
		APIVersion: store.APIVersion,
		Kind:       store.KindAnnotation,
		Metadata:   store.Metadata{ID: id, Created: store.Timestamp{Time: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)}},
		Spec: store.Spec{
			Target: store.Target{File: file},
			Type:   store.TypeWhy,
			Body:   "The test agent recorded this.",
			Anchor: store.Anchor{Scope: store.ScopeExcerpt, Excerpt: excerpt},
			Author: store.Author{Name: "Test Agent", Kind: store.AuthorAgent, Source: store.FromExplicit},
		},
		Status: store.Status{LastSeenLine: line},
	}
}
