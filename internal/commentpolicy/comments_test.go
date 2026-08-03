package commentpolicy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/policy"
	"github.com/janpuc/koment/internal/store"
)

func TestCheckAllowsIntrinsicCommentsAndRejectsExplanation(t *testing.T) {
	root := t.TempDir()
	source := `// Package sample is public API documentation.
package sample

//` + `go:generate stringer -type=State

// Exported is public API documentation.
func Exported() {}

func internal() {
	// This retries because the upstream closes idle connections.
	retry()
}
`
	writeSource(t, root, "sample.go", source)
	violations, err := Check(root, policy.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 || violations[0].Comment.Line != 10 {
		t.Fatalf("violations = %#v", violations)
	}
}

func TestAcknowledgementMustResolveAndContainTheComment(t *testing.T) {
	root := t.TempDir()
	source := "package sample\n\nfunc internal() {\n\t// Required by an external generator.\n\trun()\n}\n"
	writeSource(t, root, "sample.go", source)
	comment, err := Find("sample.go", []byte(source), "// Required by an external generator.")
	if err != nil {
		t.Fatal(err)
	}
	excerpt, err := AcknowledgementExcerpt([]byte(source), comment)
	if err != nil {
		t.Fatal(err)
	}
	captured, err := anchor.Capture([]byte(source), excerpt)
	if err != nil {
		t.Fatal(err)
	}
	record := store.Annotation{
		Version: store.RecordVersion, ID: "01JQ8ZK3M4N5P6R7S8T9V0W1X2", File: "sample.go",
		Kind: store.KindWhy, Body: "The generator reads this exact source comment.",
		Created: store.Date{Time: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)}, Anchor: captured,
		Author: store.Author{Name: "Test Human", Kind: store.AuthorHuman, Source: store.FromExplicit},
		Policy: &store.Policy{Exception: "inline-comment", Acknowledged: true},
	}
	if err := store.Open(root).Save(&record); err != nil {
		t.Fatal(err)
	}
	violations, err := Check(root, policy.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("violations = %#v", violations)
	}
}

func TestConvertRemovesCommentAndAnchorsNearbyCode(t *testing.T) {
	content := []byte("package sample\n\nfunc run() {\n\t// Retry once because the peer closes idle connections.\n\tretryOnce()\n}\n")
	comment, err := Find("sample.go", content, "// Retry once because the peer closes idle connections.")
	if err != nil {
		t.Fatal(err)
	}
	conversion, err := Convert(content, comment)
	if err != nil {
		t.Fatal(err)
	}
	if conversion.Excerpt != "retryOnce()" {
		t.Fatalf("excerpt = %q", conversion.Excerpt)
	}
	want := "package sample\n\nfunc run() {\n\tretryOnce()\n}\n"
	if string(conversion.Content) != want {
		t.Fatalf("content = %q, want %q", conversion.Content, want)
	}
	if conversion.Body != "Retry once because the peer closes idle connections." {
		t.Fatalf("body = %q", conversion.Body)
	}
}

func TestRequestedPathCannotEscapeRoot(t *testing.T) {
	_, err := Check(t.TempDir(), policy.Default(), []string{"../outside.go"})
	if err == nil {
		t.Fatal("escaping path accepted")
	}
}

func writeSource(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
