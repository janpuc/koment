package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/janpuc/koment/internal/metrics"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/store"
)

const annotatedSource = "package main\n\nfunc main() {\n\tserve()\n}\n"

func repositoryWithOneAnnotation(t *testing.T) *store.Store {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, store.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(annotatedSource), 0o644); err != nil {
		t.Fatal(err)
	}

	annotations := store.Open(root)
	excerpt := "\tserve()"
	record := &store.Record{
		Version: store.RecordVersion,
		File:    "main.go",
		Annotations: []store.Annotation{{
			ID:            "01JQ8ZK3M4N5P6R7S8T9V0W1X2",
			Scope:         store.ScopeExcerpt,
			Excerpt:       excerpt,
			ExcerptSHA256: store.ExcerptSHA256(excerpt),
			LastSeenLine:  4,
			Kind:          store.KindInvariant,
			Body:          "serve must be the last call: it blocks until the process is signalled.",
			Created:       store.Date{Time: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
		}},
	}
	if err := annotations.Save(record); err != nil {
		t.Fatal(err)
	}
	return annotations
}

func connect(t *testing.T, annotations *store.Store) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := sdk.NewInMemoryTransports()

	serverSession, err := newServer(annotations, metrics.Discard{}).Connect(ctx, serverTransport, nil)
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

func callTool[Out any](t *testing.T, session *sdk.ClientSession, name string, args map[string]any) Out {
	t.Helper()
	result, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if result.IsError {
		t.Fatalf("CallTool(%s) reported a tool error: %s", name, contentText(result))
	}

	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out Out
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatalf("decoding %s output: %v", name, err)
	}
	return out
}

func contentText(result *sdk.CallToolResult) string {
	var text strings.Builder
	for _, content := range result.Content {
		if textContent, ok := content.(*sdk.TextContent); ok {
			text.WriteString(textContent.Text)
		}
	}
	return text.String()
}

func TestServerExposesExactlyTheTwoAgreedTools(t *testing.T) {
	session := connect(t, repositoryWithOneAnnotation(t))

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	want := []string{"koment_get", "koment_search"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("ADR 0005 fixes the surface at %v, got %v", want, names)
	}
}

func TestGetReturnsResolvedAnnotations(t *testing.T) {
	session := connect(t, repositoryWithOneAnnotation(t))

	got := callTool[GetOutput](t, session, "koment_get", map[string]any{"file": "main.go"})
	if len(got.Annotations) != 1 {
		t.Fatalf("want 1 annotation, got %d", len(got.Annotations))
	}

	annotation := got.Annotations[0]
	if annotation.Status != string(anchor.StatusOK) {
		t.Errorf("want status %s, got %s", anchor.StatusOK, annotation.Status)
	}
	if annotation.Line != 4 {
		t.Errorf("want line 4, got %d", annotation.Line)
	}
	if annotation.Warning != "" {
		t.Errorf("a resolving annotation needs no warning, got %q", annotation.Warning)
	}
	if !strings.Contains(annotation.Body, "blocks until the process is signalled") {
		t.Errorf("body did not survive: %q", annotation.Body)
	}
}

func TestGetWarnsLoudlyWhenTheAnnotationHasDrifted(t *testing.T) {
	annotations := repositoryWithOneAnnotation(t)
	edited := strings.Replace(annotatedSource, "\tserve()", "\tserveForever()", 1)
	if err := os.WriteFile(filepath.Join(annotations.Root(), "main.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	session := connect(t, annotations)
	got := callTool[GetOutput](t, session, "koment_get", map[string]any{"file": "main.go"})

	annotation := got.Annotations[0]
	if annotation.Status != string(anchor.StatusDrifted) {
		t.Fatalf("want status %s, got %s", anchor.StatusDrifted, annotation.Status)
	}
	if !strings.HasPrefix(annotation.Warning, "STALE") {
		t.Errorf("ADR 0005 requires a drifted annotation to be marked as such, got warning %q", annotation.Warning)
	}
}

func TestGetReturnsNoAnnotationsForAnUnannotatedFile(t *testing.T) {
	annotations := repositoryWithOneAnnotation(t)
	if err := os.WriteFile(filepath.Join(annotations.Root(), "other.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	session := connect(t, annotations)
	got := callTool[GetOutput](t, session, "koment_get", map[string]any{"file": "other.go"})
	if len(got.Annotations) != 0 {
		t.Errorf("want no annotations, got %d", len(got.Annotations))
	}
}

func TestSearchMatchesBodiesCaseInsensitively(t *testing.T) {
	session := connect(t, repositoryWithOneAnnotation(t))

	got := callTool[SearchOutput](t, session, "koment_search", map[string]any{"query": "SIGNALLED"})
	if len(got.Matches) != 1 {
		t.Fatalf("want 1 match, got %d", len(got.Matches))
	}
	if got.Matches[0].File != "main.go" {
		t.Errorf("want main.go, got %s", got.Matches[0].File)
	}

	missing := callTool[SearchOutput](t, session, "koment_search", map[string]any{"query": "nothing here"})
	if len(missing.Matches) != 0 {
		t.Errorf("want no matches, got %d", len(missing.Matches))
	}
}

func TestSearchRejectsAnEmptyQuery(t *testing.T) {
	session := connect(t, repositoryWithOneAnnotation(t))

	result, err := session.CallTool(context.Background(),
		&sdk.CallToolParams{Name: "koment_search", Arguments: map[string]any{"query": "   "}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Error("want a tool error for an empty query")
	}
}
