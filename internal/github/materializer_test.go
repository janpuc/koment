package github

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/janpuc/koment/internal/store"
)

type roundTripFunc func(*http.Request) *http.Response

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request), nil
}

type materializerFixture struct {
	mu             sync.Mutex
	branchExists   bool
	pullExists     bool
	branchContent  []byte
	createdRecords int
	createdPulls   int
}

func (fixture *materializerFixture) response(request *http.Request) *http.Response {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	status := http.StatusOK
	var response any
	base := "/repos/example/project"
	recordPath := base + "/git/ref/heads/koment/01JQ8ZK3M4N5P6R7S8T9V0W1X2"
	switch {
	case request.Method == http.MethodGet && request.URL.Path == recordPath:
		if !fixture.branchExists {
			status = http.StatusNotFound
			response = map[string]any{"message": "Not Found"}
		} else {
			response = map[string]any{"object": map[string]any{"sha": objectID("6"), "type": "commit"}}
		}
	case request.Method == http.MethodGet && request.URL.Path == base+"/git/commits/"+objectID("a"):
		response = map[string]any{"tree": map[string]any{"sha": objectID("b")}}
	case request.Method == http.MethodGet && request.URL.Path == base+"/git/commits/"+objectID("6"):
		response = map[string]any{"tree": map[string]any{"sha": objectID("7")}}
	case request.Method == http.MethodPost && request.URL.Path == base+"/git/blobs":
		var input struct {
			Content string `json:"content"`
		}
		decodeJSONBody(request, &input)
		fixture.branchContent, _ = base64.StdEncoding.DecodeString(input.Content)
		response = map[string]any{"sha": objectID("8")}
	case request.Method == http.MethodPost && request.URL.Path == base+"/git/trees":
		response = map[string]any{"sha": objectID("7")}
	case request.Method == http.MethodPost && request.URL.Path == base+"/git/commits":
		fixture.createdRecords++
		response = map[string]any{"sha": objectID("6")}
	case request.Method == http.MethodPost && request.URL.Path == base+"/git/refs":
		fixture.branchExists = true
		response = map[string]any{"ref": "refs/heads/koment/01JQ8ZK3M4N5P6R7S8T9V0W1X2"}
	case request.Method == http.MethodGet && request.URL.Path == base+"/git/trees/"+objectID("7"):
		response = treeResponse(".koment", "tree", objectID("9"), 0)
	case request.Method == http.MethodGet && request.URL.Path == base+"/git/trees/"+objectID("9"):
		response = treeResponse("annotations", "tree", objectID("0"), 0)
	case request.Method == http.MethodGet && request.URL.Path == base+"/git/trees/"+objectID("0"):
		response = treeResponse("01JQ8ZK3M4N5P6R7S8T9V0W1X2.yaml", "blob", objectID("8"), len(fixture.branchContent))
	case request.Method == http.MethodGet && request.URL.Path == base+"/git/blobs/"+objectID("8"):
		response = encodedBlob(fixture.branchContent)
	case request.Method == http.MethodGet && request.URL.Path == base+"/pulls":
		if fixture.pullExists {
			response = []any{map[string]any{"number": 42, "html_url": "https://github.com/example/project/pull/42"}}
		} else {
			response = []any{}
		}
	case request.Method == http.MethodPost && request.URL.Path == base+"/pulls":
		fixture.pullExists = true
		fixture.createdPulls++
		response = map[string]any{"number": 42, "html_url": "https://github.com/example/project/pull/42"}
	default:
		status = http.StatusNotFound
		response = map[string]any{"message": request.Method + " " + request.URL.RequestURI()}
	}
	encoded, _ := json.Marshal(response)
	return &http.Response{
		StatusCode: status, Status: http.StatusText(status), Header: make(http.Header),
		Body: io.NopCloser(bytes.NewReader(encoded)), Request: request,
	}
}

func treeResponse(path, kind, sha string, size int) map[string]any {
	return map[string]any{"tree": []any{map[string]any{"path": path, "type": kind, "sha": sha, "size": size}}, "truncated": false}
}

func decodeJSONBody(request *http.Request, target any) {
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		panic(err)
	}
}

func materializerClient(t *testing.T, fixture *materializerFixture) *Client {
	t.Helper()
	endpoint, err := url.Parse("https://api.github.test/")
	if err != nil {
		t.Fatal(err)
	}
	return newClient(endpoint, &http.Client{Transport: roundTripFunc(fixture.response)}, "test-token")
}

func materializedRecord() store.Annotation {
	return store.Annotation{
		Version: store.RecordVersion, ID: "01JQ8ZK3M4N5P6R7S8T9V0W1X2", File: "main.go",
		Kind: store.KindWhy, Body: "The remote write is reviewed before settlement.",
		Created: store.Date{Time: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)},
		Anchor:  store.Anchor{Scope: store.ScopeExcerpt, Excerpt: "serve()", LastSeenLine: 3},
		Git:     &store.GitContext{Commit: objectID("a"), Path: "main.go", Line: 3, EndLine: 3},
		Author:  store.Author{Name: "Remote Agent", Kind: store.AuthorAgent, Source: store.FromScopedAgent, Verified: "bearer-sha256"},
	}
}

func TestMaterializeCreatesOneExactBranchCommitAndPullRequest(t *testing.T) {
	fixture := &materializerFixture{}
	client := materializerClient(t, fixture)
	first, err := client.Materialize(t.Context(), remoteRepository(), objectID("a"), materializedRecord())
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.Materialize(t.Context(), remoteRepository(), objectID("a"), materializedRecord())
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first.PullRequest != 42 || !strings.HasSuffix(first.URL, "/42") {
		t.Fatalf("first = %#v, second = %#v", first, second)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.createdRecords != 1 || fixture.createdPulls != 1 {
		t.Fatalf("created commits = %d, pull requests = %d", fixture.createdRecords, fixture.createdPulls)
	}
	wanted, err := store.EncodeAnnotation(recordPointer(materializedRecord()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fixture.branchContent, wanted) {
		t.Fatal("materialized branch did not preserve the exact encoded annotation")
	}
}

func TestMaterializeRefusesDifferentContentOnTheDeterministicBranch(t *testing.T) {
	fixture := &materializerFixture{branchExists: true, branchContent: []byte("different")}
	_, err := materializerClient(t, fixture).Materialize(t.Context(), remoteRepository(), objectID("a"), materializedRecord())
	if err == nil || !strings.Contains(err.Error(), "different content") {
		t.Fatalf("err = %v", err)
	}
}

func recordPointer(record store.Annotation) *store.Annotation {
	return &record
}
