package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	return Open(root)
}

func excerptAnnotation(id, excerpt string) Annotation {
	return Annotation{
		ID:            id,
		Scope:         ScopeExcerpt,
		Excerpt:       excerpt,
		ExcerptSHA256: ExcerptSHA256(excerpt),
		Kind:          KindGotcha,
		Body:          "An empty excerpt means file-scope, not \"matches everything\".",
		Created:       Date{time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
	}
}

func TestSaveLoadRoundTripsLosslessly(t *testing.T) {
	s := newTestStore(t)
	want := &Record{
		Version: RecordVersion,
		File:    "internal/anchor/resolve.go",
		Annotations: []Annotation{
			excerptAnnotation("01JQ8ZK3M4N5P6R7S8T9V0W1X2", "if a.Excerpt == \"\" {"),
			{
				ID:      "01JQ8ZK3M4N5P6R7S8T9V0W1X3",
				Scope:   ScopeFile,
				Kind:    KindWhy,
				Body:    "Resolution lives here rather than in store because\nthe store must not touch the working tree.",
				Created: Date{time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
			},
		},
	}

	if err := s.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load(want.File)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("round trip changed the record\n want %+v\n  got %+v", want, got)
	}
}

func TestSaveMirrorsSourcePath(t *testing.T) {
	s := newTestStore(t)
	record := &Record{
		Version:     RecordVersion,
		File:        "internal/anchor/resolve.go",
		Annotations: []Annotation{excerptAnnotation("01JQ8ZK3M4N5P6R7S8T9V0W1X2", "x")},
	}
	if err := s.Save(record); err != nil {
		t.Fatalf("Save: %v", err)
	}

	want := filepath.Join(s.Root(), DirName, "annotations", "internal", "anchor", "resolve.go.yaml")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected record at %s: %v", want, err)
	}
}

func TestSavedRecordKeepsBodyReadable(t *testing.T) {
	s := newTestStore(t)
	record := &Record{
		Version: RecordVersion,
		File:    "a.go",
		Annotations: []Annotation{{
			ID:      "01JQ8ZK3M4N5P6R7S8T9V0W1X2",
			Scope:   ScopeFile,
			Kind:    KindWhy,
			Body:    "First line of the rationale.\nSecond line of the rationale.",
			Created: Date{time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
		}},
	}
	if err := s.Save(record); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path, err := s.RecordPath("a.go")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)

	if !strings.Contains(text, "created: \"2026-07-31\"") && !strings.Contains(text, "created: 2026-07-31") {
		t.Errorf("created date not written as a plain date:\n%s", text)
	}
	if strings.Contains(text, `\n`) {
		t.Errorf("body was escaped onto one line, which defeats review in a diff:\n%s", text)
	}
}

func TestWrappedBodyIsStoredAsShortLines(t *testing.T) {
	s := newTestStore(t)
	long := "An empty excerpt means file scope, not a wildcard that matches everything. " +
		"Treating it as a wildcard made every annotation resolve at offset zero and " +
		"report ok forever, which is exactly the silent staleness koment exists to prevent."

	record := &Record{
		Version: RecordVersion,
		File:    "a.go",
		Annotations: []Annotation{{
			ID:      "01JQ8ZK3M4N5P6R7S8T9V0W1X2",
			Scope:   ScopeFile,
			Kind:    KindGotcha,
			Body:    WrapProse(long),
			Created: Date{time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
		}},
	}
	if err := s.Save(record); err != nil {
		t.Fatal(err)
	}

	path, err := s.RecordPath("a.go")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	const indent = 6
	for _, line := range strings.Split(string(content), "\n") {
		if len(line) > ProseWidth+indent {
			t.Errorf("line is %d characters, which is too wide to review in a diff: %q", len(line), line)
		}
	}

	reloaded, err := s.Load("a.go")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Annotations[0].Body != record.Annotations[0].Body {
		t.Errorf("wrapped body did not round trip\n want %q\n  got %q",
			record.Annotations[0].Body, reloaded.Annotations[0].Body)
	}
}

func TestWrapProsePreservesWordsAndParagraphs(t *testing.T) {
	original := "First paragraph that is quite long and will certainly need to be wrapped at least once, twice even.\n\nSecond paragraph."

	wrapped := WrapProse(original)
	if wrapped == original {
		t.Fatal("expected the long paragraph to be wrapped")
	}
	if got, want := strings.Join(strings.Fields(wrapped), " "), strings.Join(strings.Fields(original), " "); got != want {
		t.Errorf("wrapping changed the words\n want %q\n  got %q", want, got)
	}
	for _, line := range strings.Split(wrapped, "\n") {
		if len(line) > ProseWidth {
			t.Errorf("line exceeds %d characters: %q", ProseWidth, line)
		}
	}

	paragraphs := Paragraphs(wrapped)
	if len(paragraphs) != 2 {
		t.Fatalf("want 2 paragraphs after a round trip, got %d: %q", len(paragraphs), paragraphs)
	}
	if paragraphs[1] != "Second paragraph." {
		t.Errorf("second paragraph did not survive: %q", paragraphs[1])
	}
	if strings.Contains(paragraphs[0], "\n") {
		t.Errorf("soft wraps should be re-flowed away, got %q", paragraphs[0])
	}
}

func TestParagraphsReflowsSoftWrapsOnly(t *testing.T) {
	body := "A sentence that was\nsoft wrapped when stored.\n\nA separate paragraph."

	got := Paragraphs(body)
	want := []string{"A sentence that was soft wrapped when stored.", "A separate paragraph."}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestLoadReportsMissingRecordAsNotExist(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Load("never/annotated.go")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("want fs.ErrNotExist, got %v", err)
	}
}

func TestLoadRejectsCorruptedExcerptHash(t *testing.T) {
	s := newTestStore(t)
	annotation := excerptAnnotation("01JQ8ZK3M4N5P6R7S8T9V0W1X2", "original")
	record := &Record{Version: RecordVersion, File: "a.go", Annotations: []Annotation{annotation}}
	if err := s.Save(record); err != nil {
		t.Fatal(err)
	}

	path, err := s.RecordPath("a.go")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(content), "excerpt: original", "excerpt: tampered", 1)
	if tampered == string(content) {
		t.Fatal("test did not tamper with the excerpt")
	}
	if err := os.WriteFile(path, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Load("a.go"); err == nil {
		t.Fatal("want an error when the stored excerpt no longer matches its hash, got nil")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	s := newTestStore(t)
	path, err := s.RecordPath("a.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	record := "version: 1\nfile: a.go\nannotations:\n  - id: X\n    scope: file\n    kind: why\n    body: b\n    created: 2026-07-31\n    confidence: high\n"
	if err := os.WriteFile(path, []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Load("a.go"); err == nil {
		t.Fatal("want an error for an unknown field, got nil")
	}
}

func TestLoadRejectsRecordStoredUnderTheWrongPath(t *testing.T) {
	s := newTestStore(t)
	path, err := s.RecordPath("a.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	record := "version: 1\nfile: elsewhere.go\nannotations:\n  - id: X\n    scope: file\n    kind: why\n    body: b\n    created: 2026-07-31\n"
	if err := os.WriteFile(path, []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = s.Load("a.go")
	if err == nil || !strings.Contains(err.Error(), "elsewhere.go") {
		t.Fatalf("want an error naming the mismatched path, got %v", err)
	}
}

func TestValidateRejectsBadAnnotations(t *testing.T) {
	base := excerptAnnotation("01JQ8ZK3M4N5P6R7S8T9V0W1X2", "snippet")

	cases := map[string]func(a *Annotation){
		"empty excerpt with excerpt scope": func(a *Annotation) { a.Excerpt = ""; a.ExcerptSHA256 = "" },
		"excerpt carried by file scope":    func(a *Annotation) { a.Scope = ScopeFile },
		"unknown kind":                     func(a *Annotation) { a.Kind = "todo" },
		"unknown scope":                    func(a *Annotation) { a.Scope = "symbol" },
		"blank body":                       func(a *Annotation) { a.Body = "   " },
		"missing id":                       func(a *Annotation) { a.ID = "" },
		"missing created":                  func(a *Annotation) { a.Created = Date{} },
	}

	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			annotation := base
			corrupt(&annotation)
			if err := annotation.Validate(); err == nil {
				t.Fatal("want an error, got nil")
			}
		})
	}
}

func TestValidateRejectsDuplicateIDs(t *testing.T) {
	duplicate := excerptAnnotation("01JQ8ZK3M4N5P6R7S8T9V0W1X2", "snippet")
	record := Record{
		Version:     RecordVersion,
		File:        "a.go",
		Annotations: []Annotation{duplicate, duplicate},
	}
	err := record.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want a duplicate id error, got %v", err)
	}
}

func TestRecordPathRejectsEscapingPaths(t *testing.T) {
	s := newTestStore(t)
	for _, path := range []string{"../outside.go", "a/../../outside.go", "/etc/passwd", ""} {
		if _, err := s.RecordPath(path); err == nil {
			t.Errorf("RecordPath(%q) should have failed", path)
		}
	}
}

func TestAnnotatedFilesListsEveryRecordSorted(t *testing.T) {
	s := newTestStore(t)
	for _, file := range []string{"z.go", "internal/a.go", "cmd/koment/main.go"} {
		record := &Record{
			Version:     RecordVersion,
			File:        file,
			Annotations: []Annotation{excerptAnnotation("01JQ8ZK3M4N5P6R7S8T9V0W1X2", "x")},
		}
		if err := s.Save(record); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.AnnotatedFiles()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"cmd/koment/main.go", "internal/a.go", "z.go"}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestAnnotatedFilesIsEmptyBeforeAnythingIsAnnotated(t *testing.T) {
	s := newTestStore(t)
	got, err := s.AnnotatedFiles()
	if err != nil {
		t.Fatalf("want no error for a store with no annotations, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want no files, got %v", got)
	}
}

func TestFindRootPrefersKomentOverGit(t *testing.T) {
	outer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outer, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(outer, "nested")
	if err := os.MkdirAll(filepath.Join(inner, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(inner, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindRoot(deep)
	if err != nil {
		t.Fatal(err)
	}
	if got != inner {
		t.Errorf("want %s, got %s", inner, got)
	}
}

func TestFindRootFallsBackToGitWorkTree(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindRoot(deep)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Errorf("want %s, got %s", root, got)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
