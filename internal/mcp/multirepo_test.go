package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/janpuc/koment/internal/repository"
	"github.com/janpuc/koment/internal/store"
)

func twoRepositories(t *testing.T) *repository.Set {
	t.Helper()
	base := t.TempDir()

	for _, fixture := range []struct{ id, recordID, body string }{
		{"api", "01JQ8ZK3M4N5P6R7S8T9V0W1X2", "serve must be the last call in the API; it blocks until signalled."},
		{"web", "01JQ8ZK3M4N5P6R7S8T9V0W1X3", "the web handler must not block; it returns before the render finishes."},
	} {
		root := filepath.Join(base, fixture.id)
		if err := os.MkdirAll(filepath.Join(root, store.DirName), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(annotatedSource), 0o644); err != nil {
			t.Fatal(err)
		}

		annotations := store.Open(root)
		excerpt := "\tserve()"
		if err := annotations.Save(&store.Annotation{
			Version: store.RecordVersion,
			ID:      fixture.recordID,
			File:    "main.go",
			Anchor: store.Anchor{
				Scope:        store.ScopeExcerpt,
				Excerpt:      excerpt,
				LastSeenLine: 4,
			},
			Kind:    store.KindInvariant,
			Body:    fixture.body,
			Created: store.Date{Time: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)},
			Author:  store.Author{Name: "Test Agent", Kind: store.AuthorAgent, Source: store.FromExplicit},
		}); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv(repository.EnvRepos, "api="+filepath.Join(base, "api")+",web="+filepath.Join(base, "web"))
	set, err := repository.Load(base)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func TestRepositoriesToolListsWhatIsServed(t *testing.T) {
	session := connectTo(t, twoRepositories(t))

	got := callTool[RepositoriesOutput](t, session, "koment_repositories", map[string]any{})
	if len(got.Repositories) != 2 {
		t.Fatalf("want 2 repositories, got %d", len(got.Repositories))
	}

	byID := map[string]RepositorySummary{}
	for _, entry := range got.Repositories {
		byID[entry.ID] = entry
	}
	for _, id := range []string{"api", "web"} {
		entry, found := byID[id]
		if !found {
			t.Fatalf("%s missing from the listing", id)
		}
		if entry.Files != 1 || entry.Annotations["ok"] != 1 {
			t.Errorf("%s counts wrong: %+v", id, entry)
		}
	}
}

func TestGetRefusesAnAmbiguousPathAndNamesTheCandidates(t *testing.T) {
	session := connectTo(t, twoRepositories(t))

	result, err := session.CallTool(context.Background(),
		&sdk.CallToolParams{Name: "koment_get", Arguments: map[string]any{"file": "main.go"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("a path in two repositories must not be answered from one of them silently")
	}

	message := contentText(result)
	for _, want := range []string{"api", "web", "repository"} {
		if !strings.Contains(message, want) {
			t.Errorf("the error should name %q so the caller can choose; got: %s", want, message)
		}
	}
}

func TestGetUsesTheNamedRepository(t *testing.T) {
	session := connectTo(t, twoRepositories(t))

	for _, want := range []struct{ repository, body string }{
		{"api", "blocks until signalled"},
		{"web", "must not block"},
	} {
		got := callTool[GetOutput](t, session, "koment_get",
			map[string]any{"file": "main.go", "repository": want.repository})

		if got.Repository != want.repository {
			t.Errorf("want repository %s, got %s", want.repository, got.Repository)
		}
		if len(got.Annotations) != 1 {
			t.Fatalf("want 1 annotation from %s, got %d", want.repository, len(got.Annotations))
		}
		if !strings.Contains(got.Annotations[0].Body, want.body) {
			t.Errorf("got the wrong repository's annotation: %q", got.Annotations[0].Body)
		}
		if got.Annotations[0].Repository != want.repository {
			t.Errorf("every annotation must carry its repository, got %q", got.Annotations[0].Repository)
		}
	}
}

func TestGetRejectsAnUnknownRepository(t *testing.T) {
	session := connectTo(t, twoRepositories(t))

	result, err := session.CallTool(context.Background(), &sdk.CallToolParams{
		Name: "koment_get", Arguments: map[string]any{"file": "main.go", "repository": "nope"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("an unknown repository must fail rather than fall back")
	}
	if !strings.Contains(contentText(result), "served:") {
		t.Errorf("the error should list what is served; got: %s", contentText(result))
	}
}

func TestSearchSpansEveryRepositoryUnlessNarrowed(t *testing.T) {
	session := connectTo(t, twoRepositories(t))

	all := callTool[SearchOutput](t, session, "koment_search", map[string]any{"query": "block"})
	if len(all.Matches) != 2 {
		t.Fatalf("an unqualified search should span repositories, got %d", len(all.Matches))
	}
	seen := map[string]bool{}
	for _, match := range all.Matches {
		seen[match.Repository] = true
		if match.Repository == "" {
			t.Error("every match must name its repository")
		}
	}
	if !seen["api"] || !seen["web"] {
		t.Errorf("want matches from both repositories, got %v", seen)
	}

	narrowed := callTool[SearchOutput](t, session, "koment_search",
		map[string]any{"query": "block", "repository": "api"})
	if len(narrowed.Matches) != 1 || narrowed.Matches[0].Repository != "api" {
		t.Errorf("a narrowed search should stay in one repository, got %+v", narrowed.Matches)
	}
}

func TestSingleRepositoryNeedsNoRepositoryArgument(t *testing.T) {
	session := connect(t, repositoryWithOneAnnotation(t))

	got := callTool[GetOutput](t, session, "koment_get", map[string]any{"file": "main.go"})
	if len(got.Annotations) != 1 {
		t.Fatalf("one repository should resolve without being named, got %d", len(got.Annotations))
	}
	if got.Repository == "" {
		t.Error("the response should still say which repository answered")
	}
}
