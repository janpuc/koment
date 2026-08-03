package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janpuc/koment/internal/store"
)

const source = "package main\n\nfunc main() {\n\tserve()\n}\n"

const rationale = "serve must be the last call: it blocks until the process is signalled."

func annotatedRepository(t *testing.T) *store.Store {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, store.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o644); err != nil {
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
			Body:          store.WrapProse(rationale),
			Created:       store.Date{Time: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
		}},
	}
	if err := annotations.Save(record); err != nil {
		t.Fatal(err)
	}
	return annotations
}

func get(t *testing.T, annotations *store.Store, target string) (int, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	Handler(annotations).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder.Code, recorder.Body.String()
}

func TestIndexRendersCodeAndItsAnnotationTogether(t *testing.T) {
	code, body := get(t, annotatedRepository(t), "/")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}

	for _, want := range []string{"main.go", "func main()", "serve()", rationale, "invariant"} {
		if !strings.Contains(body, want) {
			t.Errorf("page is missing %q", want)
		}
	}
	if !strings.Contains(body, "1 ok") {
		t.Errorf("tally missing from the page:\n%s", firstLines(body, 40))
	}
}

func TestAnnotationIsAnchoredToItsLine(t *testing.T) {
	_, body := get(t, annotatedRepository(t), "/f/main.go")

	marked := strings.Index(body, `class="row marked`)
	if marked < 0 {
		t.Fatal("no line was marked as annotated")
	}
	rest := body[marked:]
	if !strings.Contains(rest[:min(len(rest), 900)], rationale) {
		t.Error("the annotation is not rendered alongside the line it anchors to")
	}
	if !strings.Contains(body, `<div class="ln">4</div>`) {
		t.Error("line 4 was not rendered")
	}
}

func TestDriftIsRenderedAsHistoryNotAsCurrentCode(t *testing.T) {
	annotations := annotatedRepository(t)
	edited := strings.Replace(source, "\tserve()", "\tserveForever()", 1)
	if err := os.WriteFile(filepath.Join(annotations.Root(), "main.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	_, body := get(t, annotations, "/f/main.go")

	if !strings.Contains(body, "note drifted stale") {
		t.Error("a drifted annotation must be marked stale")
	}
	if !strings.Contains(body, "excerpt no longer in the file") {
		t.Error("the vanished excerpt must be labelled as gone")
	}
	if !strings.Contains(body, "<s>") {
		t.Error("the vanished excerpt must be struck through")
	}
	if !strings.Contains(body, "Treat it as history") {
		t.Error("the warning must say what the status costs the reader")
	}
	if !strings.Contains(body, "1 drifted") {
		t.Error("the tally must show the drift")
	}
}

func TestOrphanedFileStillShowsItsAnnotations(t *testing.T) {
	annotations := annotatedRepository(t)
	if err := os.Remove(filepath.Join(annotations.Root(), "main.go")); err != nil {
		t.Fatal(err)
	}

	code, body := get(t, annotations, "/f/main.go")
	if code != http.StatusOK {
		t.Fatalf("want 200 for an orphan, got %d", code)
	}
	if !strings.Contains(body, "file is gone") {
		t.Error("the page must say the source is gone")
	}
	if !strings.Contains(body, rationale) {
		t.Error("an orphaned annotation is still readable history and must be shown")
	}
	if strings.Contains(body, "func main()") {
		t.Error("no source should be rendered for a file that does not exist")
	}
}

func TestCodeIsEscapedRatherThanInterpreted(t *testing.T) {
	annotations := annotatedRepository(t)
	hostile := "package main\n\nfunc main() {\n\tserve() // <script>alert(1)</script>\n}\n"
	if err := os.WriteFile(filepath.Join(annotations.Root(), "main.go"), []byte(hostile), 0o644); err != nil {
		t.Fatal(err)
	}

	_, body := get(t, annotations, "/f/main.go")
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("source content must be escaped, not injected into the page")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("the escaped form should still be visible as code")
	}
}

func TestUnannotatedFileSaysSoRatherThan404(t *testing.T) {
	code, body := get(t, annotatedRepository(t), "/f/never/touched.go")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	if !strings.Contains(body, "Not annotated") {
		t.Errorf("want a plain statement, got:\n%s", firstLines(body, 30))
	}
}

func TestEmptyRepositoryExplainsHowToStart(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, store.DirName), 0o755); err != nil {
		t.Fatal(err)
	}

	code, body := get(t, store.Open(root), "/")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	if !strings.Contains(body, "No annotations yet") || !strings.Contains(body, "koment add") {
		t.Errorf("the empty state should tell you what to do:\n%s", firstLines(body, 30))
	}
}

func TestStylesheetIsServedFromTheBinary(t *testing.T) {
	code, body := get(t, annotatedRepository(t), "/assets/style.css")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	if !strings.Contains(body, "--drifted") {
		t.Error("stylesheet does not look like koment's")
	}
}

func TestEveryStatusHasAColour(t *testing.T) {
	_, css := get(t, annotatedRepository(t), "/assets/style.css")
	for _, status := range []string{"ok", "moved", "drifted", "orphaned"} {
		if !strings.Contains(css, ".dot."+status) || !strings.Contains(css, ".pill."+status) {
			t.Errorf("status %q has no visual treatment", status)
		}
	}
}

func firstLines(body string, n int) string {
	lines := strings.Split(body, "\n")
	return strings.Join(lines[:min(len(lines), n)], "\n")
}
