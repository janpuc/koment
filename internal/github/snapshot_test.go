package github

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/serving"
)

func objectID(character string) string { return strings.Repeat(character, 40) }

func remoteRepository() serving.Repository {
	return serving.Repository{
		Identity: application.RepositoryIdentity{ID: "project", Name: "Project"},
		Provider: "github", Remote: "example/project", Branch: "main", Default: true,
	}
}

type githubFixture struct {
	mu           sync.Mutex
	authorized   bool
	sourceExists bool
	sourceSize   int64
}

func (fixture *githubFixture) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	fixture.mu.Lock()
	fixture.authorized = fixture.authorized || request.Header.Get("Authorization") == "Bearer test-token"
	fixture.mu.Unlock()
	writer.Header().Set("Content-Type", "application/json")
	response := any(nil)
	switch request.URL.Path {
	case "/repos/example/project/git/ref/heads/main":
		response = map[string]any{"object": map[string]any{"sha": objectID("a"), "type": "commit"}}
	case "/repos/example/project/git/commits/" + objectID("a"):
		response = map[string]any{"tree": map[string]any{"sha": objectID("b")}}
	case "/repos/example/project/git/trees/" + objectID("b"):
		entries := []any{map[string]any{"path": ".koment", "type": "tree", "sha": objectID("c")}}
		if fixture.sourceExists {
			entries = append(entries, map[string]any{"path": "src", "type": "tree", "sha": objectID("f")})
		}
		response = map[string]any{"tree": entries, "truncated": false}
	case "/repos/example/project/git/trees/" + objectID("c"):
		response = map[string]any{"tree": []any{map[string]any{"path": "annotations", "type": "tree", "sha": objectID("d")}}, "truncated": false}
	case "/repos/example/project/git/trees/" + objectID("d"):
		response = map[string]any{"tree": []any{map[string]any{
			"path": "01JQ8ZK3M4N5P6R7S8T9V0W1X2.yaml", "type": "blob", "sha": objectID("e"), "size": int64(len(annotationFixture())),
		}}, "truncated": false}
	case "/repos/example/project/git/blobs/" + objectID("e"):
		response = encodedBlob(annotationFixture())
	case "/repos/example/project/git/trees/" + objectID("f"):
		response = map[string]any{"tree": []any{map[string]any{
			"path": "main.go", "type": "blob", "sha": objectID("1"), "size": fixture.sourceSize,
		}}, "truncated": false}
	case "/repos/example/project/git/blobs/" + objectID("1"):
		response = encodedBlob([]byte("package sample\n\nvar Remote = true\n"))
	default:
		http.Error(writer, `{"message":"not found"}`, http.StatusNotFound)
		return
	}
	if err := json.NewEncoder(writer).Encode(response); err != nil {
		panic(err)
	}
}

func annotationFixture() []byte {
	return []byte("version: 1\nid: 01JQ8ZK3M4N5P6R7S8T9V0W1X2\nfile: src/main.go\nkind: why\nbody: The remote value is the compatibility default.\ncreated: 2026-08-03\nanchor:\n  scope: excerpt\n  excerpt: var Remote = true\n  last_seen_line: 3\nauthor:\n  name: Test Agent\n  kind: agent\n  source: explicit\n")
}

func encodedBlob(content []byte) map[string]any {
	return map[string]any{
		"content": base64.StdEncoding.EncodeToString(content), "encoding": "base64", "size": len(content),
	}
}

func fixtureClient(t *testing.T, fixture *githubFixture) *Client {
	t.Helper()
	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	return newClient(endpoint, server.Client(), "test-token")
}

func TestSnapshotReadsOneCommitAndResolvesItsSource(t *testing.T) {
	source := []byte("package sample\n\nvar Remote = true\n")
	fixture := &githubFixture{sourceExists: true, sourceSize: int64(len(source))}
	snapshot, err := fixtureClient(t, fixture).Snapshot(t.Context(), remoteRepository())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Commit != objectID("a") || snapshot.Repository.CloneURL != "https://github.com/example/project" {
		t.Fatalf("metadata = %#v", snapshot)
	}
	if len(snapshot.Files) != 1 || snapshot.Files[0].Annotations[0].Status != anchor.StatusOK {
		t.Fatalf("files = %#v", snapshot.Files)
	}
	fixture.mu.Lock()
	authorized := fixture.authorized
	fixture.mu.Unlock()
	if !authorized {
		t.Fatal("GitHub requests were not authenticated")
	}
}

func TestSnapshotPreservesAnOrphanWhenTheSourceBlobIsMissing(t *testing.T) {
	snapshot, err := fixtureClient(t, &githubFixture{}).Snapshot(t.Context(), remoteRepository())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Files[0].Exists || snapshot.Files[0].Annotations[0].Status != anchor.StatusOrphaned {
		t.Fatalf("file = %#v", snapshot.Files[0])
	}
}

func TestSnapshotRejectsADeclaredSourceLargerThanItsBound(t *testing.T) {
	fixture := &githubFixture{sourceExists: true, sourceSize: maximumSource + 1}
	_, err := fixtureClient(t, fixture).Snapshot(t.Context(), remoteRepository())
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("err = %v", err)
	}
}

func TestSnapshotRejectsAnUnscopedRemoteBeforeMakingARequest(t *testing.T) {
	repository := remoteRepository()
	repository.Remote = "https://attacker.invalid/example/project"
	_, err := fixtureClient(t, &githubFixture{}).Snapshot(t.Context(), repository)
	if err == nil || !strings.Contains(err.Error(), "owner/name") {
		t.Fatalf("err = %v", err)
	}
}
