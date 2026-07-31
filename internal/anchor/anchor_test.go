package anchor

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/janpuc/koment/internal/store"
)

const anchoredExcerpt = "\tif excerpt == \"\" {"

const excerptLineInBefore = 6

func annotation(excerpt string, lastSeenLine int) store.Annotation {
	return store.Annotation{
		ID:            "01JQ8ZK3M4N5P6R7S8T9V0W1X2",
		Scope:         store.ScopeExcerpt,
		Excerpt:       excerpt,
		ExcerptSHA256: store.ExcerptSHA256(excerpt),
		LastSeenLine:  lastSeenLine,
		Kind:          store.KindGotcha,
		Body:          "An empty excerpt means file scope, not a wildcard.",
		Created:       store.Date{Time: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
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

// resolveAcross writes the before file, confirms the anchor is ok, then
// replaces it with the after file and resolves again.
func resolveAcross(t *testing.T, before, after string) Resolution {
	t.Helper()
	source := filepath.Join(t.TempDir(), "resolve.go")

	if err := os.WriteFile(source, testdata(t, before), 0o644); err != nil {
		t.Fatal(err)
	}
	record := &store.Record{
		Version:     store.RecordVersion,
		File:        "resolve.go",
		Annotations: []store.Annotation{annotation(anchoredExcerpt, excerptLineInBefore)},
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("the before-state record is not valid: %v", err)
	}

	initial, err := ResolveRecord(record, source)
	if err != nil {
		t.Fatal(err)
	}
	if initial[0].Status != StatusOK {
		t.Fatalf("before state should resolve %s, got %s at line %d", StatusOK, initial[0].Status, initial[0].Line)
	}

	if err := os.WriteFile(source, testdata(t, after), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveRecord(record, source)
	if err != nil {
		t.Fatal(err)
	}
	return resolved[0]
}

func TestUneditedExcerptResolvesOK(t *testing.T) {
	got := resolveAcross(t, "before.go.txt", "before.go.txt")
	if got.Status != StatusOK {
		t.Errorf("want %s, got %s", StatusOK, got.Status)
	}
	if got.Line != excerptLineInBefore {
		t.Errorf("want line %d, got %d", excerptLineInBefore, got.Line)
	}
}

func TestCodeInsertedAboveResolvesMoved(t *testing.T) {
	got := resolveAcross(t, "before.go.txt", "after-moved.go.txt")
	if got.Status != StatusMoved {
		t.Errorf("want %s, got %s", StatusMoved, got.Status)
	}
	if want := 10; got.Line != want {
		t.Errorf("want line %d, got %d", want, got.Line)
	}
	if got.Status.IsFailure() {
		t.Error("moved must not fail a check")
	}
}

func TestReformattingElsewhereStillResolves(t *testing.T) {
	got := resolveAcross(t, "before.go.txt", "after-reformatted-elsewhere.go.txt")
	if got.Status.IsFailure() {
		t.Errorf("reformatting away from the excerpt must not break the anchor, got %s", got.Status)
	}
	if want := 8; got.Line != want {
		t.Errorf("want line %d, got %d", want, got.Line)
	}
}

func TestEditingTheExcerptResolvesDrifted(t *testing.T) {
	got := resolveAcross(t, "before.go.txt", "after-drifted.go.txt")
	if got.Status != StatusDrifted {
		t.Errorf("want %s, got %s", StatusDrifted, got.Status)
	}
	if !got.Status.IsFailure() {
		t.Error("drifted must fail a check")
	}
}

func TestDeletingTheFileResolvesOrphaned(t *testing.T) {
	source := filepath.Join(t.TempDir(), "gone.go")
	record := &store.Record{
		Version:     store.RecordVersion,
		File:        "gone.go",
		Annotations: []store.Annotation{annotation(anchoredExcerpt, excerptLineInBefore)},
	}

	resolved, err := ResolveRecord(record, source)
	if err != nil {
		t.Fatalf("a missing file is a status, not an error: %v", err)
	}
	if resolved[0].Status != StatusOrphaned {
		t.Errorf("want %s, got %s", StatusOrphaned, resolved[0].Status)
	}
	if !resolved[0].Status.IsFailure() {
		t.Error("orphaned must fail a check")
	}
}

func TestFileScopeResolvesOKWhateverTheContent(t *testing.T) {
	fileScoped := store.Annotation{
		ID:      "01JQ8ZK3M4N5P6R7S8T9V0W1X3",
		Scope:   store.ScopeFile,
		Kind:    store.KindWhy,
		Body:    "This file is generated; edit the template instead.",
		Created: store.Date{Time: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
	}
	got := Resolve(fileScoped, testdata(t, "after-drifted.go.txt"))
	if got.Status != StatusOK {
		t.Errorf("want %s, got %s", StatusOK, got.Status)
	}
	if got.Line != 0 {
		t.Errorf("file scope points at no line, got %d", got.Line)
	}
}

func TestFileScopeResolvesOrphanedWhenTheFileIsGone(t *testing.T) {
	fileScoped := store.Annotation{
		ID:      "01JQ8ZK3M4N5P6R7S8T9V0W1X3",
		Scope:   store.ScopeFile,
		Kind:    store.KindWhy,
		Body:    "This file is generated; edit the template instead.",
		Created: store.Date{Time: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
	}
	record := &store.Record{Version: store.RecordVersion, File: "gone.go", Annotations: []store.Annotation{fileScoped}}

	resolved, err := ResolveRecord(record, filepath.Join(t.TempDir(), "gone.go"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].Status != StatusOrphaned {
		t.Errorf("want %s, got %s", StatusOrphaned, resolved[0].Status)
	}
}

func TestMissingLastSeenLineResolvesOKRatherThanMoved(t *testing.T) {
	got := Resolve(annotation(anchoredExcerpt, 0), testdata(t, "after-moved.go.txt"))
	if got.Status != StatusOK {
		t.Errorf("an unknown previous position is not evidence of movement, got %s", got.Status)
	}
	if want := 10; got.Line != want {
		t.Errorf("want line %d, got %d", want, got.Line)
	}
}

func TestRepeatedExcerptPrefersLastSeenLine(t *testing.T) {
	content := []byte("a\nx\nb\nx\nc\nx\n")

	atSecond := Resolve(annotation("x", 4), content)
	if atSecond.Status != StatusOK || atSecond.Line != 4 {
		t.Errorf("want %s at line 4, got %s at line %d", StatusOK, atSecond.Status, atSecond.Line)
	}
	if want := 3; atSecond.Occurrences != want {
		t.Errorf("want %d occurrences, got %d", want, atSecond.Occurrences)
	}

	nowhereNear := Resolve(annotation("x", 5), content)
	if nowhereNear.Status != StatusMoved || nowhereNear.Line != 2 {
		t.Errorf("want %s at line 2, got %s at line %d", StatusMoved, nowhereNear.Status, nowhereNear.Line)
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
