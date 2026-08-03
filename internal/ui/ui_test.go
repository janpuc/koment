package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/repository"
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
	record := &store.Annotation{
		Version: store.RecordVersion,
		ID:      "01JQ8ZK3M4N5P6R7S8T9V0W1X2",
		File:    "main.go",
		Anchor: store.Anchor{
			Scope:        store.ScopeExcerpt,
			Excerpt:      excerpt,
			LastSeenLine: 4,
		},
		Kind:    store.KindInvariant,
		Body:    store.WrapProse(rationale),
		Created: store.Date{Time: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
		Author:  store.Author{Name: "Test Human", Kind: store.AuthorHuman, Source: store.FromExplicit},
	}
	if err := annotations.Save(record); err != nil {
		t.Fatal(err)
	}
	return annotations
}

func setOf(annotations *store.Store) *repository.Set {
	return repository.Of(repository.Repository{ID: "test", Root: annotations.Root()})
}

func snapshotFromStore(t *testing.T, annotations *store.Store) *application.RepositorySnapshot {
	t.Helper()
	built, err := application.BuildSnapshot(repository.Repository{ID: "test", Name: "fixture", Root: annotations.Root()})
	if err != nil {
		t.Fatal(err)
	}
	return built
}

func request(t *testing.T, repositories *repository.Set, target string) (int, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	Handler(repositories).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder.Code, recorder.Body.String()
}

func get(t *testing.T, annotations *store.Store, page string) (int, string) {
	t.Helper()
	return request(t, setOf(annotations), "/r/test/"+strings.TrimPrefix(page, "/"))
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

	if !strings.Contains(body, `class="row marked`) {
		t.Error("no line was marked as annotated")
	}
	if !strings.Contains(body, `id="L4"`) {
		t.Error("line 4 was not rendered with an addressable anchor")
	}
	if !strings.Contains(body, `data-for="4"`) {
		t.Error("the note does not say which line it belongs to")
	}
	if !strings.Contains(body, rationale) {
		t.Error("the rationale is missing from the page")
	}
}

// The measured defect: a note rendered inside its line's grid row stretched
// that row, so ten annotations turned a five-line const block into thirty-five
// rendered lines. Lines and notes are now separate columns.
func TestNotesAreNotInTheCodeGridSoTheyCannotPushCodeApart(t *testing.T) {
	annotations := annotatedRepository(t)
	_, body := get(t, annotations, "/f/main.go")

	code := between(t, body, `<div class="code">`, `<aside class="gloss"`)
	if strings.Contains(code, `class="note `) {
		t.Error("a note rendered inside the code block stretches the line it sits on")
	}
	if !strings.Contains(body, `<aside class="gloss"`) {
		t.Error("the notes have no column of their own to float in")
	}
}

func TestManyAnnotationsOnOneLineDoNotStretchTheCode(t *testing.T) {
	annotations := annotatedRepository(t)
	crowd(t, annotations, 10)

	_, body := get(t, annotations, "/f/main.go")

	if got := strings.Count(body, `class="row`); got != 5 {
		t.Errorf("the file has 5 lines; the page rendered %d rows", got)
	}
	if got := strings.Count(body, `data-for="4"`); got != 11 {
		t.Errorf("want 11 notes against line 4, got %d", got)
	}
}

func crowd(t *testing.T, annotations *store.Store, extra int) {
	t.Helper()
	excerpt := "\tserve()"
	for i := range extra {
		id, err := store.NewID(time.Date(2026, 8, 3, 0, 0, i, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		record := store.Annotation{
			Version: store.RecordVersion,
			ID:      id,
			File:    "main.go",
			Anchor: store.Anchor{
				Scope:        store.ScopeExcerpt,
				Excerpt:      excerpt,
				LastSeenLine: 4,
			},
			Kind:    store.KindWhy,
			Body:    store.WrapProse(strings.Repeat("Reasoning that runs long enough to be several lines tall. ", 4)),
			Created: store.Date{Time: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)},
			Author:  store.Author{Name: "Test Human", Kind: store.AuthorHuman, Source: store.FromExplicit},
		}
		if err := annotations.Save(&record); err != nil {
			t.Fatal(err)
		}
	}
}

func between(t *testing.T, body, from, to string) string {
	t.Helper()
	start := strings.Index(body, from)
	if start < 0 {
		t.Fatalf("missing %q in the page", from)
	}
	rest := body[start:]
	if end := strings.Index(rest, to); end > 0 {
		return rest[:end]
	}
	return rest
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
	if !strings.Contains(body, "Treat this as history") {
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
	code, body := request(t, setOf(annotatedRepository(t)), "/assets/style.css")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	if !strings.Contains(body, "--drifted") {
		t.Error("stylesheet does not look like koment's")
	}
}

func TestBrandAssetsAreServedFromTheBinary(t *testing.T) {
	configured := setOf(annotatedRepository(t))
	for _, asset := range []string{"/assets/koment-logo.svg", "/assets/koment-logo.png"} {
		code, _ := request(t, configured, asset)
		if code != http.StatusOK {
			t.Errorf("%s: want 200, got %d", asset, code)
		}
	}

	_, page := get(t, annotatedRepository(t), "/")
	if !strings.Contains(page, `src="/assets/koment-logo.svg"`) ||
		!strings.Contains(page, `href="/assets/koment-logo.png"`) {
		t.Error("page does not link the logo and PNG favicon")
	}
}

func TestEveryStatusHasAColour(t *testing.T) {
	_, css := request(t, setOf(annotatedRepository(t)), "/assets/style.css")
	for _, status := range []string{"ok", "moved", "ambiguous", "drifted", "orphaned"} {
		if !strings.Contains(css, ".dot."+status) || !strings.Contains(css, ".pill."+status) {
			t.Errorf("status %q has no visual treatment", status)
		}
	}
}

func TestWriteModeRequiresCapabilityOriginAndFormToken(t *testing.T) {
	annotations := annotatedRepository(t)
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "UI Human"},
		{"config", "user.email", "ui@example.test"},
	} {
		command := exec.Command("git", args...)
		command.Dir = annotations.Root()
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	writeHandler := handler(setOf(annotations), "secret-capability")

	bootstrap := httptest.NewRecorder()
	writeHandler.ServeHTTP(bootstrap, httptest.NewRequest(http.MethodGet, "/?"+capabilityQuery+"=secret-capability", nil))
	if bootstrap.Code != http.StatusSeeOther || len(bootstrap.Result().Cookies()) != 1 {
		t.Fatalf("bootstrap = %d, cookies = %#v", bootstrap.Code, bootstrap.Result().Cookies())
	}
	cookie := bootstrap.Result().Cookies()[0]
	if bootstrap.Header().Get("Location") != "/" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("bootstrap location = %q, cookie = %#v", bootstrap.Header().Get("Location"), cookie)
	}

	pageRequest := httptest.NewRequest(http.MethodGet, "/r/test/f/main.go", nil)
	pageRequest.AddCookie(cookie)
	page := httptest.NewRecorder()
	writeHandler.ServeHTTP(page, pageRequest)
	if !strings.Contains(page.Body.String(), "Add rationale") {
		t.Fatalf("write form unavailable:\n%s", page.Body.String())
	}

	form := url.Values{
		"capability": {"secret-capability"}, "file": {"main.go"}, "kind": {"why"},
		"excerpt": {"func main()"}, "body": {"The entry point remains orchestration-only."},
	}
	withoutOrigin := httptest.NewRequest(http.MethodPost, "/r/test/annotations", strings.NewReader(form.Encode()))
	withoutOrigin.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	withoutOrigin.AddCookie(cookie)
	refused := httptest.NewRecorder()
	writeHandler.ServeHTTP(refused, withoutOrigin)
	if refused.Code != http.StatusForbidden {
		t.Fatalf("missing Origin returned %d", refused.Code)
	}

	request := httptest.NewRequest(http.MethodPost, "/r/test/annotations", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.com")
	request.AddCookie(cookie)
	created := httptest.NewRecorder()
	writeHandler.ServeHTTP(created, request)
	if created.Code != http.StatusSeeOther {
		t.Fatalf("write returned %d: %s", created.Code, created.Body.String())
	}
	records, err := annotations.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1].Author.Name != "UI Human" || records[1].Author.Kind != store.AuthorHuman {
		t.Fatalf("records = %#v", records)
	}
}

func TestReadOnlyHandlerNeverRendersMutationForm(t *testing.T) {
	_, body := get(t, annotatedRepository(t), "/f/main.go")
	if strings.Contains(body, "Add rationale") {
		t.Fatal("read-only handler exposed mutation form")
	}
}

func TestFileLinksEscapeURLControlCharactersBySegment(t *testing.T) {
	got := servedLinks("test").file("nested/a #?.go")
	if got != "/r/test/f/nested/a%20%23%3F.go" {
		t.Fatalf("file link = %q", got)
	}
}

func firstLines(body string, n int) string {
	lines := strings.Split(body, "\n")
	return strings.Join(lines[:min(len(lines), n)], "\n")
}
