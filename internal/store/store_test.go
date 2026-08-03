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

const (
	firstID  = "01JQ8ZK3M4N5P6R7S8T9V0W1X2"
	secondID = "01JQ8ZK3M4N5P6R7S8T9V0W1X3"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	return Open(root)
}

func testAuthor() Author {
	return Author{Name: "Test Human", Kind: AuthorHuman, Source: FromExplicit}
}

func excerptAnnotation(id, file, excerpt string) Annotation {
	return Annotation{
		Version: RecordVersion,
		ID:      id,
		File:    file,
		Kind:    KindGotcha,
		Body:    "An empty excerpt means file scope, not a wildcard.",
		Created: Date{Time: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
		Anchor: Anchor{
			Scope:        ScopeExcerpt,
			Excerpt:      excerpt,
			LastSeenLine: 1,
		},
		Author: testAuthor(),
	}
}

func fileAnnotation(id, file string) Annotation {
	annotation := excerptAnnotation(id, file, "unused")
	annotation.Kind = KindWhy
	annotation.Anchor = Anchor{Scope: ScopeFile}
	return annotation
}

func TestSaveLoadRoundTripsLosslessly(t *testing.T) {
	annotations := newTestStore(t)
	want := excerptAnnotation(firstID, "internal/anchor/resolve.go", "if anchor.Excerpt == \"\" {")
	want.Anchor.Before = "func Resolve(anchor Anchor) Resolution {\n\tif ready {"
	want.Anchor.After = "\treturn Resolution{}\n}"

	if err := annotations.Save(&want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := annotations.Load(want.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(want, *got) {
		t.Errorf("round trip changed the annotation\n want %+v\n  got %+v", want, *got)
	}
}

func TestSaveUsesOneFlatFilePerAnnotationID(t *testing.T) {
	annotations := newTestStore(t)
	first := excerptAnnotation(firstID, "internal/anchor/resolve.go", "first")
	second := excerptAnnotation(secondID, "internal/anchor/resolve.go", "second")
	for _, annotation := range []*Annotation{&first, &second} {
		if err := annotations.Save(annotation); err != nil {
			t.Fatal(err)
		}
	}

	for _, id := range []string{firstID, secondID} {
		want := filepath.Join(annotations.Root(), DirName, annotationsDir, id+recordSuffix)
		if _, err := os.Stat(want); err != nil {
			t.Fatalf("expected annotation at %s: %v", want, err)
		}
	}
}

func TestSavedRecordStartsWithSchemaDirectiveAndKeepsBodyReadable(t *testing.T) {
	annotations := newTestStore(t)
	record := fileAnnotation(firstID, "a.go")
	record.Body = "First line of the rationale.\nSecond line of the rationale."
	if err := annotations.Save(&record); err != nil {
		t.Fatal(err)
	}

	path, err := annotations.RecordPath(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	content := readFile(t, path)
	if !strings.HasPrefix(content, schemaDirective) {
		t.Errorf("record does not start with the schema directive:\n%s", content)
	}
	if !strings.Contains(content, "created: \"2026-07-31\"") && !strings.Contains(content, "created: 2026-07-31") {
		t.Errorf("created date not written as a plain date:\n%s", content)
	}
	if strings.Contains(content, `\n`) {
		t.Errorf("body was escaped onto one line:\n%s", content)
	}
}

func TestWrappedBodyIsStoredAsShortLines(t *testing.T) {
	annotations := newTestStore(t)
	long := "An empty excerpt means file scope, not a wildcard that matches everything. " +
		"Treating it as a wildcard made every annotation resolve at offset zero and " +
		"report ok forever, which is exactly the silent staleness koment exists to prevent."
	record := fileAnnotation(firstID, "a.go")
	record.Body = WrapProse(long)
	if err := annotations.Save(&record); err != nil {
		t.Fatal(err)
	}

	path, err := annotations.RecordPath(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(readFile(t, path), "\n") {
		if strings.HasPrefix(line, "# yaml-language-server:") {
			continue
		}
		if len(line) > ProseWidth+2 {
			t.Errorf("line is %d characters, which is too wide to review: %q", len(line), line)
		}
	}

	reloaded, err := annotations.Load(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Body != record.Body {
		t.Errorf("wrapped body did not round trip\n want %q\n  got %q", record.Body, reloaded.Body)
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
	if len(paragraphs) != 2 || paragraphs[1] != "Second paragraph." {
		t.Fatalf("paragraphs did not survive: %q", paragraphs)
	}
}

func TestLoadReportsMissingRecordAsNotExist(t *testing.T) {
	annotations := newTestStore(t)
	_, err := annotations.Load(firstID)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("want fs.ErrNotExist, got %v", err)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	annotations := newTestStore(t)
	path, err := annotations.RecordPath(firstID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	record := "version: 1\nid: " + firstID + "\nfile: a.go\nkind: why\nbody: b\ncreated: 2026-07-31\nanchor:\n  scope: file\nauthor:\n  name: Test\n  kind: human\n  source: explicit\nconfidence: high\n"
	if err := os.WriteFile(path, []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := annotations.Load(firstID); err == nil {
		t.Fatal("want an error for an unknown field")
	}
}

func TestLoadRejectsASecondYAMLDocument(t *testing.T) {
	annotations := newTestStore(t)
	record := fileAnnotation(firstID, "a.go")
	if err := annotations.Save(&record); err != nil {
		t.Fatal(err)
	}
	path, err := annotations.RecordPath(firstID)
	if err != nil {
		t.Fatal(err)
	}
	content := readFile(t, path) + "---\nversion: 1\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := annotations.Load(firstID); err == nil || !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("want a multiple-document error, got %v", err)
	}
}

func TestLoadRejectsRecordStoredUnderTheWrongID(t *testing.T) {
	annotations := newTestStore(t)
	record := fileAnnotation(secondID, "a.go")
	path, err := annotations.RecordPath(firstID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "version: 1\nid: " + record.ID + "\nfile: a.go\nkind: why\nbody: body\ncreated: 2026-07-31\nanchor:\n  scope: file\nauthor:\n  name: Test\n  kind: human\n  source: explicit\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = annotations.Load(firstID)
	if err == nil || !strings.Contains(err.Error(), secondID) {
		t.Fatalf("want an error naming the mismatched id, got %v", err)
	}
}

func TestValidateRejectsBadAnnotations(t *testing.T) {
	base := excerptAnnotation(firstID, "a.go", "snippet")
	cases := map[string]func(*Annotation){
		"wrong version":         func(annotation *Annotation) { annotation.Version = 2 },
		"invalid id":            func(annotation *Annotation) { annotation.ID = "X" },
		"empty file":            func(annotation *Annotation) { annotation.File = "" },
		"noncanonical file":     func(annotation *Annotation) { annotation.File = "./a.go" },
		"backslash file":        func(annotation *Annotation) { annotation.File = `internal\a.go` },
		"drive-relative file":   func(annotation *Annotation) { annotation.File = "C:a.go" },
		"escaping file":         func(annotation *Annotation) { annotation.File = "../a.go" },
		"empty excerpt":         func(annotation *Annotation) { annotation.Anchor.Excerpt = "" },
		"excerpt on file":       func(annotation *Annotation) { annotation.Anchor.Scope = ScopeFile },
		"unknown kind":          func(annotation *Annotation) { annotation.Kind = "todo" },
		"unknown scope":         func(annotation *Annotation) { annotation.Anchor.Scope = "symbol" },
		"blank body":            func(annotation *Annotation) { annotation.Body = "   " },
		"missing created":       func(annotation *Annotation) { annotation.Created = Date{} },
		"missing author":        func(annotation *Annotation) { annotation.Author = Author{} },
		"too much context":      func(annotation *Annotation) { annotation.Anchor.Before = "1\n2\n3\n4" },
		"unacknowledged policy": func(annotation *Annotation) { annotation.Policy = &Policy{Exception: "inline-comment"} },
	}

	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			annotation := base
			corrupt(&annotation)
			if err := annotation.Validate(); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestRecordPathRejectsInvalidIDs(t *testing.T) {
	annotations := newTestStore(t)
	for _, id := range []string{"../outside", "lowercase000000000000000000", "", "81JQ8ZK3M4N5P6R7S8T9V0W1X"} {
		if _, err := annotations.RecordPath(id); err == nil {
			t.Errorf("RecordPath(%q) should have failed", id)
		}
	}
}

func TestForFileAndAnnotatedFilesUseRecordContent(t *testing.T) {
	annotations := newTestStore(t)
	records := []Annotation{
		fileAnnotation(firstID, "z.go"),
		fileAnnotation(secondID, "internal/a.go"),
	}
	for index := range records {
		if err := annotations.Save(&records[index]); err != nil {
			t.Fatal(err)
		}
	}

	files, err := annotations.AnnotatedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"internal/a.go", "z.go"}; !reflect.DeepEqual(want, files) {
		t.Errorf("want %v, got %v", want, files)
	}
	onlyZ, err := annotations.ForFile("z.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyZ) != 1 || onlyZ[0].ID != firstID {
		t.Errorf("want %s for z.go, got %+v", firstID, onlyZ)
	}
}

func TestConcurrentSavesToTheSameSourceKeepBothRecords(t *testing.T) {
	annotations := newTestStore(t)
	first := fileAnnotation(firstID, "shared.go")
	second := fileAnnotation(secondID, "shared.go")
	errors := make(chan error, 2)
	for _, record := range []*Annotation{&first, &second} {
		go func() { errors <- annotations.Save(record) }()
	}
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}

	stored, err := annotations.ForFile("shared.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 {
		t.Fatalf("want both concurrent records, got %+v", stored)
	}
}

func TestReadSourceAllowsOnlySymlinksThatStayInsideTheRoot(t *testing.T) {
	annotations := newTestStore(t)
	inside := filepath.Join(annotations.Root(), "inside.go")
	if err := os.WriteFile(inside, []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("inside.go", filepath.Join(annotations.Root(), "alias.go")); err != nil {
		t.Fatal(err)
	}
	content, err := annotations.ReadSource("alias.go")
	if err != nil || string(content) != "inside" {
		t.Fatalf("an in-root symlink should be readable, got %q and %v", content, err)
	}

	outside := filepath.Join(t.TempDir(), "secret.go")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(annotations.Root(), "escape.go")); err != nil {
		t.Fatal(err)
	}
	content, err = annotations.ReadSource("escape.go")
	if err == nil || string(content) == "secret" {
		t.Fatalf("an escaping symlink returned %q and %v", content, err)
	}
}

func TestWriteSourceIsAtomicRootedAndPreservesPermissions(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "script.go")
	if err := os.WriteFile(file, []byte("before"), 0o744); err != nil {
		t.Fatal(err)
	}
	annotations := Open(root)
	if err := annotations.WriteSource("script.go", []byte("after")); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "after" {
		t.Fatalf("content = %q", content)
	}
	information, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if information.Mode().Perm() != 0o744 {
		t.Fatalf("mode = %o", information.Mode().Perm())
	}

	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.go")); err != nil {
		t.Fatal(err)
	}
	if err := annotations.WriteSource("escape.go", []byte("escaped")); err == nil {
		t.Fatal("write followed a symlink outside the repository root")
	}
}

func TestSaveCannotFollowAnAnnotationDirectoryOutsideTheRoot(t *testing.T) {
	annotations := newTestStore(t)
	outside := t.TempDir()
	annotationDirectory := filepath.Join(annotations.Root(), DirName, annotationsDir)
	if err := os.Symlink(outside, annotationDirectory); err != nil {
		t.Fatal(err)
	}
	record := fileAnnotation(firstID, "a.go")
	if err := annotations.Save(&record); err == nil {
		t.Fatal("saving through an escaping annotations symlink should fail")
	}
	if _, err := os.Stat(filepath.Join(outside, firstID+recordSuffix)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("save wrote outside the repository: %v", err)
	}
}

func TestAnnotatedFilesIsEmptyBeforeAnythingIsAnnotated(t *testing.T) {
	annotations := newTestStore(t)
	got, err := annotations.AnnotatedFiles()
	if err != nil || len(got) != 0 {
		t.Fatalf("want no annotated files, got %v and %v", got, err)
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
