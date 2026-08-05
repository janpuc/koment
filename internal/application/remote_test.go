package application

import (
	"strings"
	"testing"

	"github.com/janpuc/koment/internal/store"
)

func TestDraftAnnotationAnchorsAndAttributesTheExactSnapshot(t *testing.T) {
	snapshot := &RepositorySnapshot{
		Repository: RepositoryIdentity{ID: "api"}, Commit: strings.Repeat("a", 40),
		Files: []FileSnapshot{{Path: "main.go", Exists: true, Content: []byte("package main\n\nserve()\n")}},
	}
	record, err := DraftAnnotation(snapshot, AddInput{
		File: "main.go", Excerpt: "serve()", Kind: store.TypeWhy, Body: "The call owns the process lifetime.",
		Author: store.Author{Name: "Remote Agent", Kind: store.AuthorAgent, Source: store.FromScopedAgent, Verified: "bearer-sha256"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Status.LastSeenLine != 3 || record.Spec.Git.Commit != snapshot.Commit || record.Spec.Git.Path != "main.go" {
		t.Fatalf("record = %#v", record)
	}
}

func TestDraftAnnotationRefusesMissingOrAmbiguousSource(t *testing.T) {
	snapshot := &RepositorySnapshot{
		Repository: RepositoryIdentity{ID: "api"}, Commit: strings.Repeat("a", 40),
		Files: []FileSnapshot{{Path: "main.go", Exists: true, Content: []byte("serve()\nserve()\n")}},
	}
	author := store.Author{Name: "Remote Agent", Kind: store.AuthorAgent, Source: store.FromScopedAgent}
	for _, input := range []AddInput{
		{File: "absent.go", Kind: store.TypeWhy, Body: "Missing", Author: author},
		{File: "main.go", Excerpt: "serve()", Kind: store.TypeWhy, Body: "Ambiguous", Author: author},
	} {
		if _, err := DraftAnnotation(snapshot, input); err == nil {
			t.Errorf("accepted %#v", input)
		}
	}
}
