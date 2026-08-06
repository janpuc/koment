package anchor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/koment-dev/koment/internal/store"
)

const anchoredExcerpt = "\tif excerpt == \"\" {"

const excerptLineInBefore = 6

func annotation(excerpt string, lastSeenLine int) store.Annotation {
	return store.Annotation{
		APIVersion: store.APIVersion,
		Kind:       store.KindAnnotation,
		Metadata:   store.Metadata{ID: "01JQ8ZK3M4N5P6R7S8T9V0W1X2", Created: store.Timestamp{Time: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)}},
		Spec: store.Spec{
			Target: store.Target{File: "resolve.go"},
			Type:   store.TypeGotcha,
			Body:   "An empty excerpt means file scope, not a wildcard.",
			Anchor: store.Anchor{Scope: store.ScopeExcerpt, Excerpt: excerpt},
			Author: store.Author{Name: "Test Agent", Kind: store.AuthorAgent, Source: store.FromExplicit},
		},
		Status: store.Status{LastSeenLine: lastSeenLine},
	}
}

func testdata(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func resolveAcross(t *testing.T, before, after string) Resolution {
	return resolveAcrossExcerpt(t, before, after, anchoredExcerpt)
}

func resolveAcrossExcerpt(t *testing.T, before, after, excerpt string) Resolution {
	t.Helper()
	source := filepath.Join(t.TempDir(), "resolve.go")
	beforeContent := testdata(t, before)
	if err := os.WriteFile(source, beforeContent, 0o644); err != nil {
		t.Fatal(err)
	}
	record := annotation(excerpt, excerptLineInBefore)
	captured, _, err := Capture(beforeContent, excerpt)
	if err != nil {
		t.Fatal(err)
	}
	record.Spec.Anchor = captured
	if err := record.Validate(); err != nil {
		t.Fatalf("the before-state record is not valid: %v", err)
	}

	initial, err := ResolveRecord(record, source)
	if err != nil {
		t.Fatal(err)
	}
	if initial.Status != StatusOK {
		t.Fatalf("before state should resolve %s, got %s at line %d", StatusOK, initial.Status, initial.Line)
	}

	if err := os.WriteFile(source, testdata(t, after), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveRecord(record, source)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestUneditedExcerptResolvesOK(t *testing.T) {
	got := resolveAcross(t, "before.go.txt", "before.go.txt")
	if got.Status != StatusOK || got.Line != excerptLineInBefore {
		t.Errorf("want %s at line %d, got %s at line %d", StatusOK, excerptLineInBefore, got.Status, got.Line)
	}
}

// Resolution is a search, so an annotation follows its excerpt down the file on
// its own. The recorded line is not consulted and its staleness is not a status
// a reader has to act on (ADR 0116).
func TestCodeInsertedAboveStillResolvesCleanlyAtTheNewLine(t *testing.T) {
	got := resolveAcross(t, "before.go.txt", "after-moved.go.txt")
	if got.Status != StatusOK {
		t.Errorf("want %s, got %s", StatusOK, got.Status)
	}
	if got.Line != 10 {
		t.Errorf("want the excerpt found at line 10, got %d", got.Line)
	}
	if got.Line == got.Annotation.Status.LastSeenLine {
		t.Fatal("this proves nothing unless the recorded line is stale")
	}
}

func TestReformattingElsewhereStillResolves(t *testing.T) {
	got := resolveAcross(t, "before.go.txt", "after-reformatted-elsewhere.go.txt")
	if got.Status.IsFailure() || got.Line != 8 {
		t.Errorf("want a successful resolution at line 8, got %s at line %d", got.Status, got.Line)
	}
}

func TestEditingTheExcerptResolvesDrifted(t *testing.T) {
	got := resolveAcross(t, "before.go.txt", "after-drifted.go.txt")
	if got.Status != StatusDrifted || !got.Status.IsFailure() {
		t.Errorf("want failing %s, got %s", StatusDrifted, got.Status)
	}
}

func TestDeletingTheFileResolvesOrphaned(t *testing.T) {
	record := annotation(anchoredExcerpt, excerptLineInBefore)
	resolved, err := ResolveRecord(record, filepath.Join(t.TempDir(), "gone.go"))
	if err != nil {
		t.Fatalf("a missing file is a status, not an error: %v", err)
	}
	if resolved.Status != StatusOrphaned || !resolved.Status.IsFailure() {
		t.Errorf("want failing %s, got %s", StatusOrphaned, resolved.Status)
	}
}

func TestFileScopeResolvesOKWhateverTheContent(t *testing.T) {
	fileScoped := annotation("unused", 1)
	fileScoped.Spec.Anchor = store.Anchor{Scope: store.ScopeFile}
	got := Resolve(fileScoped, testdata(t, "after-drifted.go.txt"))
	if got.Status != StatusOK || got.Line != 0 {
		t.Errorf("want file-scoped %s at no line, got %s at line %d", StatusOK, got.Status, got.Line)
	}
}

func TestContextDisambiguatesARepeatedExcerpt(t *testing.T) {
	got := resolveAcrossExcerpt(t, "before-repeated.go.txt", "after-contextual.go.txt", "\ttarget()")
	if got.Status != StatusOK || got.Line != 7 || got.Occurrences != 2 {
		t.Errorf("want %s at line 7 among 2 occurrences, got %s at line %d among %d",
			StatusOK, got.Status, got.Line, got.Occurrences)
	}
}

func TestIdenticalContextResolvesAmbiguous(t *testing.T) {
	got := resolveAcrossExcerpt(t, "before-repeated.go.txt", "after-ambiguous.go.txt", "\ttarget()")
	if got.Status != StatusAmbiguous || !got.Status.IsFailure() || got.Occurrences != 2 {
		t.Errorf("want failing %s with 2 occurrences, got %s with %d", StatusAmbiguous, got.Status, got.Occurrences)
	}
}

func TestLastSeenLineNeverChoosesAmongRepeatedExcerpts(t *testing.T) {
	content := testdata(t, "after-ambiguous.go.txt")
	record := annotation("\ttarget()", 17)
	record.Spec.Anchor.Before = "\tprepareOne()\n\tprepareTwo()\n\tprepareThree()"
	record.Spec.Anchor.After = "\tfinishOne()\n\tfinishTwo()\n\tfinishThree()"
	got := Resolve(record, content)
	if got.Status != StatusAmbiguous {
		t.Errorf("last_seen_line must not select a candidate, got %s at line %d", got.Status, got.Line)
	}
}

func TestCaptureRejectsARepeatedExcerpt(t *testing.T) {
	_, _, err := Capture(testdata(t, "after-contextual.go.txt"), "\ttarget()")
	if err == nil || !strings.Contains(err.Error(), "2 times") {
		t.Fatalf("want an actionable repeated-excerpt error, got %v", err)
	}
}

func TestCaptureStoresAtMostThreeCompleteLinesOfContext(t *testing.T) {
	captured, _, err := Capture(testdata(t, "before-repeated.go.txt"), "\ttarget()")
	if err != nil {
		t.Fatal(err)
	}
	if want := "\tprepareOne()\n\tprepareTwo()\n\tprepareThree()"; captured.Before != want {
		t.Errorf("want before context %q, got %q", want, captured.Before)
	}
	if want := "\tfinishOne()\n\tfinishTwo()\n\tfinishThree()"; captured.After != want {
		t.Errorf("want after context %q, got %q", want, captured.After)
	}
}

func TestExcerptLinesCountsEveryOccurrence(t *testing.T) {
	content := []byte("alpha\nbeta\nalpha\n\nalpha")
	got := ExcerptLines(content, "alpha")
	want := []int{1, 3, 5}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestExcerptLinesHandlesMultiLineExcerpts(t *testing.T) {
	content := testdata(t, "before.go.txt")
	multiLine := "\tif excerpt == \"\" {\n\t\treturn false\n\t}"
	got := ExcerptLines(content, multiLine)
	want := []int{excerptLineInBefore}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestExcerptLinesFindsNothingForAnEmptyExcerpt(t *testing.T) {
	if got := ExcerptLines([]byte("anything"), ""); got != nil {
		t.Errorf("an empty excerpt must match nothing, got %v", got)
	}
}
