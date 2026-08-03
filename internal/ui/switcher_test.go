package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janpuc/koment/internal/repository"
	"github.com/janpuc/koment/internal/store"
)

func twoRepositories(t *testing.T) *repository.Set {
	t.Helper()
	base := t.TempDir()

	for _, fixture := range []struct{ id, body string }{
		{"api", "serve is last in the API: it blocks until signalled."},
		{"web", "the web handler must not block; it returns before the render finishes."},
	} {
		root := filepath.Join(base, fixture.id)
		if err := os.MkdirAll(filepath.Join(root, store.DirName), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}

		annotations := store.Open(root)
		excerpt := "\tserve()"
		if err := annotations.Save(&store.Record{
			Version: store.RecordVersion,
			File:    "main.go",
			Annotations: []store.Annotation{{
				ID:            "01JQ8ZK3M4N5P6R7S8T9V0W1X" + strings.ToUpper(fixture.id[:1]),
				Scope:         store.ScopeExcerpt,
				Excerpt:       excerpt,
				ExcerptSHA256: store.ExcerptSHA256(excerpt),
				LastSeenLine:  4,
				Kind:          store.KindInvariant,
				Body:          store.WrapProse(fixture.body),
				Created:       store.Date{Time: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)},
			}},
		}); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv(repository.EnvRepos, "api="+filepath.Join(base, "api")+",web="+filepath.Join(base, "web"))
	set, err := repository.Load(base)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// The UI serves every configured repository rather than refusing like
// koment_get does: a person can see which one they are looking at, so there is
// no ambiguity to refuse (ADR 0025, ADR 0026).
func TestSeveralRepositoriesAreAllServed(t *testing.T) {
	repositories := twoRepositories(t)

	for _, want := range []struct{ id, body string }{
		{"api", "blocks until signalled"},
		{"web", "must not block"},
	} {
		code, page := request(t, repositories, "/r/"+want.id+"/f/main.go")
		if code != http.StatusOK {
			t.Fatalf("%s: want 200, got %d", want.id, code)
		}
		if !strings.Contains(page, want.body) {
			t.Errorf("%s served the wrong repository's annotation", want.id)
		}
	}
}

func TestTheSwitcherOffersEveryRepositoryAndMarksTheCurrentOne(t *testing.T) {
	_, page := request(t, twoRepositories(t), "/r/api/")

	for _, want := range []string{`value="/r/api/"`, `value="/r/web/"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the switcher is missing %s", want)
		}
	}
	if !strings.Contains(page, `value="/r/api/" selected`) {
		t.Error("the switcher does not mark which repository is being shown")
	}
	// The switcher must work with scripting off, or the published tier loses
	// navigation it has no server to replace.
	if !strings.Contains(page, "<noscript>") || !strings.Contains(page, `href="/r/web/"`) {
		t.Error("there is no scriptless way to change repository")
	}
}

func TestASingleRepositoryGetsNoSwitcher(t *testing.T) {
	_, page := get(t, annotatedRepository(t), "/")

	if strings.Contains(page, "data-switcher") {
		t.Error("a control offering one choice is noise")
	}
}

func TestTheRootRedirectsIntoARepository(t *testing.T) {
	recorder := httptest.NewRecorder()
	Handler(twoRepositories(t)).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusFound {
		t.Fatalf("want a redirect from /, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Location"); got != "/r/api/" {
		t.Errorf("want /r/api/, got %q", got)
	}
}

// Falling back to some other repository would answer a question nobody asked
// with data from a repository they did not mean.
func TestAnUnknownRepositoryIsRefusedAndNamesWhatIsServed(t *testing.T) {
	code, page := request(t, twoRepositories(t), "/r/nope/")

	if code != http.StatusNotFound {
		t.Fatalf("want 404 for an unknown repository, got %d", code)
	}
	for _, want := range []string{"api", "web"} {
		if !strings.Contains(page, want) {
			t.Errorf("the refusal should list %q so the reader can choose; got: %s", want, page)
		}
	}
}

// The served path reaches os.ReadFile, so the only thing standing between a
// crafted URL and the rest of the disk is store.SourcePath. gosec cannot see
// through it (.golangci.yml excludes G703 here); this is what makes that safe.
func TestAServedPathCannotEscapeTheRepository(t *testing.T) {
	annotations := annotatedRepository(t)
	outside := filepath.Join(filepath.Dir(annotations.Root()), "outside.txt")
	if err := os.WriteFile(outside, []byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, escape := range []string{
		"../outside.txt",
		"../../outside.txt",
		"internal/../../outside.txt",
		"/etc/passwd",
	} {
		code, page := request(t, setOf(annotations), "/r/test/f/"+escape)
		if strings.Contains(page, "SECRET") {
			t.Fatalf("%q escaped the repository root and served %d", escape, code)
		}
	}
}

// A URL that means a different repository depending on how the server was
// configured is a URL nobody can share.
func TestEvenOneRepositoryIsAddressedByName(t *testing.T) {
	annotations := annotatedRepository(t)

	code, _ := request(t, setOf(annotations), "/r/test/f/main.go")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	if code, _ := request(t, setOf(annotations), "/f/main.go"); code == http.StatusOK {
		t.Error("an unscoped path must not resolve; every URL carries its repository")
	}
}
