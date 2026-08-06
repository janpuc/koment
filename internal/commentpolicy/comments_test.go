package commentpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koment-dev/koment/internal/anchor"
	"github.com/koment-dev/koment/internal/policy"
	"github.com/koment-dev/koment/internal/store"
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
	captured, capturedLine, err := anchor.Capture([]byte(source), excerpt)
	if err != nil {
		t.Fatal(err)
	}
	record := store.Annotation{
		APIVersion: store.APIVersion,
		Kind:       store.KindAnnotation,
		Metadata:   store.Metadata{ID: "01JQ8ZK3M4N5P6R7S8T9V0W1X2", Created: store.Timestamp{Time: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)}},
		Spec: store.Spec{
			Target: store.Target{File: "sample.go"},
			Type:   store.TypeWhy,
			Body:   "The generator reads this exact source comment.",
			Anchor: captured,
			Author: store.Author{Name: "Test Human", Kind: store.AuthorHuman, Source: store.FromExplicit},
			Policy: &store.Policy{Exception: "inline-comment", Acknowledged: true},
		},
		Status: store.Status{LastSeenLine: capturedLine},
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

func TestCheckContentUsesUnsavedSource(t *testing.T) {
	content := []byte("package sample\n\nfunc run() {\n\t// Explain the retry.\n\tretry()\n}\n")
	violations, err := CheckContent("sample.go", content, policy.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 || violations[0].Comment.Raw != "// Explain the retry." {
		t.Fatalf("violations = %#v", violations)
	}
}

func TestCommentIntentDoesNotAutoConvertCommentedCode(t *testing.T) {
	prose := SourceComment{File: "sample.go", Body: "Retry because the peer closes idle connections."}
	code := SourceComment{File: "sample.go", Body: "retry()"}
	if !IsCommentIntent(prose) {
		t.Fatal("explanatory prose was not classified as comment intent")
	}
	if IsCommentIntent(code) {
		t.Fatal("commented-out code was classified as comment intent")
	}
}

func writeSource(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A run of adjacent line comments is one thought, so it is one violation and
// converts as one annotation. Treating each line separately would ask the
// author the same question once per line and split the prose across records.
func TestAdjacentLineCommentsAreOneCommentNotSeveral(t *testing.T) {
	root := t.TempDir()
	source := `package sample

func internal() {
	// The upstream closes idle connections after thirty seconds,
	// so a pooled connection can be dead before the first write.
	// Retrying once is cheaper than probing every borrow.
	retry()
}
`
	writeSource(t, root, "sample.go", source)
	violations, err := Check(root, policy.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 {
		t.Fatalf("want one violation for the group, got %d: %#v", len(violations), violations)
	}
	comment := violations[0].Comment
	for _, fragment := range []string{"thirty seconds", "dead before the first write", "cheaper than probing"} {
		if !strings.Contains(comment.Body, fragment) {
			t.Errorf("the group lost %q; body = %q", fragment, comment.Body)
		}
	}
	if !IsCommentIntent(comment) {
		t.Error("a three-line explanation should read as comment intent")
	}

	converted, err := Convert([]byte(source), comment)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(converted.Content), "thirty seconds") {
		t.Error("converting left part of the group in the source")
	}
	if !strings.Contains(converted.Body, "cheaper than probing") {
		t.Error("converting dropped the last line of the group")
	}
}
