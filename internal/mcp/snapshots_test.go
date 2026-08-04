package mcp

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/metrics"
	"github.com/janpuc/koment/internal/serving"
	"github.com/janpuc/koment/internal/store"
)

type recordingMaterializer struct {
	mu     sync.Mutex
	record store.Annotation
}

func (materializer *recordingMaterializer) Materialize(
	_ context.Context, _ serving.Repository, _ string, record store.Annotation,
) (serving.Materialization, error) {
	materializer.mu.Lock()
	defer materializer.mu.Unlock()
	materializer.record = record
	return serving.Materialization{
		Branch: "koment/" + record.ID, Commit: servedCommit, PullRequest: 42,
		URL: "https://github.com/example/api/pull/42",
	}, nil
}

const servedCommit = "0123456789abcdef0123456789abcdef01234567"

func servedCatalog(t *testing.T) *serving.Catalog {
	t.Helper()
	repositories := []serving.Repository{
		{Identity: application.RepositoryIdentity{ID: "api", Name: "Payments API"}, Provider: "github", Remote: "example/api", Branch: "main", Default: true},
		{Identity: application.RepositoryIdentity{ID: "web", Name: "Customer Web"}, Provider: "github", Remote: "example/web", Branch: "main"},
	}
	catalog, err := serving.NewCatalog(repositories)
	if err != nil {
		t.Fatal(err)
	}
	for index, repository := range repositories {
		record := store.Annotation{
			Version: store.RecordVersion, ID: []string{"01JQ8ZK3M4N5P6R7S8T9V0W1X2", "01JQ8ZK3M4N5P6R7S8T9V0W1X3"}[index], File: "main.go",
			Anchor: store.Anchor{Scope: store.ScopeExcerpt, Excerpt: "serve()", LastSeenLine: 3},
			Kind:   store.KindWhy, Body: "The provider snapshot is immutable.",
			Created: store.Date{Time: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)},
			Author:  store.Author{Name: "Test Agent", Kind: store.AuthorAgent, Source: store.FromExplicit},
		}
		snapshot, assembleErr := application.AssembleSnapshot(application.SnapshotInput{
			Repository: repository.Identity, Commit: servedCommit, Records: []store.Annotation{record},
			Sources: map[string][]byte{"main.go": []byte("package main\n\nserve()\n")},
		})
		if assembleErr != nil {
			t.Fatal(assembleErr)
		}
		if replaceErr := catalog.Replace(snapshot); replaceErr != nil {
			t.Fatal(replaceErr)
		}
	}
	return catalog
}

func connectSnapshots(t *testing.T, access RepositoryAccess) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	serverSession, err := NewSnapshotServer(servedCatalog(t), metrics.Discard{}, access).Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverSession.Close() })
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clientSession.Close() })
	return clientSession
}

func TestSnapshotToolsReturnTheExactServedCommit(t *testing.T) {
	session := connectSnapshots(t, RepositoryAccess{"api": true})
	listed := callTool[RepositoriesOutput](t, session, "koment_repositories", map[string]any{})
	if len(listed.Repositories) != 1 || listed.Repositories[0].Commit != servedCommit {
		t.Fatalf("repositories = %+v", listed.Repositories)
	}
	got := callTool[GetOutput](t, session, "koment_get", map[string]any{"file": "main.go"})
	if got.Repository != "api" || got.Commit != servedCommit || len(got.Annotations) != 1 {
		t.Fatalf("get = %+v", got)
	}
	if got.Annotations[0].Commit != servedCommit {
		t.Errorf("annotation commit = %q", got.Annotations[0].Commit)
	}
}

func TestSnapshotToolsCannotReadOutsideTheirRepositoryScope(t *testing.T) {
	session := connectSnapshots(t, RepositoryAccess{"api": true})
	result, err := session.CallTool(context.Background(), &sdk.CallToolParams{
		Name: "koment_get", Arguments: map[string]any{"repository": "web", "file": "main.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || !strings.Contains(contentText(result), "accessible") {
		t.Fatalf("result = %#v, content = %q", result, contentText(result))
	}
}

func TestSnapshotGetRejectsPathsThatCouldEscapeARepository(t *testing.T) {
	session := connectSnapshots(t, nil)
	for _, path := range []string{"../secret", "/etc/passwd", "a/../../secret"} {
		result, err := session.CallTool(context.Background(), &sdk.CallToolParams{
			Name: "koment_get", Arguments: map[string]any{"repository": "api", "file": path},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError {
			t.Errorf("%q should fail", path)
		}
	}
}

func TestWritableSnapshotServerMaterializesAnAttributedPullRequest(t *testing.T) {
	ctx := context.Background()
	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	materializer := &recordingMaterializer{}
	author := store.Author{
		Name: "Release Agent", Kind: store.AuthorAgent, Source: store.FromScopedAgent, Verified: "bearer-sha256",
	}
	serverSession, err := NewWritableSnapshotServer(
		servedCatalog(t), metrics.Discard{}, RepositoryAccess{"api": true}, author, materializer,
	).Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverSession.Close() })
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	got := callTool[MutationOutput](t, session, "koment_add", map[string]any{
		"file": "main.go", "excerpt": "serve()", "kind": "why", "body": "The server owns process lifetime.",
	})
	if got.Review == nil || got.Review.PullRequest != 42 || got.Review.BaseCommit != servedCommit {
		t.Fatalf("mutation = %#v", got)
	}
	materializer.mu.Lock()
	defer materializer.mu.Unlock()
	if materializer.record.Author != author || materializer.record.Git.Commit != servedCommit {
		t.Fatalf("record = %#v", materializer.record)
	}
}
