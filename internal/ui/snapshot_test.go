package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/koment-dev/koment/internal/application"
	"github.com/koment-dev/koment/internal/serving"
	"github.com/koment-dev/koment/internal/store"
)

const snapshotCommit = "0123456789abcdef0123456789abcdef01234567"

func synchronizedCatalog(t *testing.T) *serving.Catalog {
	t.Helper()
	repositories := []serving.Repository{
		{Identity: application.RepositoryIdentity{ID: "api", Name: "Payments API"}, Provider: "github", Remote: "example/api", Branch: "main", Default: true},
		{Identity: application.RepositoryIdentity{ID: "web", Name: "Customer Web"}, Provider: "github", Remote: "example/web", Branch: "main"},
	}
	catalog, err := serving.NewCatalog(repositories)
	if err != nil {
		t.Fatal(err)
	}
	for _, repository := range repositories {
		record := store.Annotation{
			APIVersion: store.APIVersion,
			Kind:       store.KindAnnotation,
			Metadata:   store.Metadata{ID: "01JQ8ZK3M4N5P6R7S8T9V0W1X2", Created: store.Timestamp{Time: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)}},
			Spec: store.Spec{
				Target: store.Target{File: "main.go"},
				Type:   store.TypeWhy,
				Body:   "The provider snapshot is immutable.",
				Anchor: store.Anchor{Scope: store.ScopeExcerpt, Excerpt: "serve()"},
				Author: store.Author{Name: "Test Agent", Kind: store.AuthorAgent, Source: store.FromExplicit},
			},
			Status: store.Status{LastSeenLine: 3},
		}
		snapshot, assembleErr := application.AssembleSnapshot(application.SnapshotInput{
			Repository: repository.Identity, Commit: snapshotCommit, Records: []store.Annotation{record},
			Sources: map[string][]byte{"main.go": []byte("package main\n\nserve()\n")},
		})
		if assembleErr != nil {
			t.Fatal(assembleErr)
		}
		if replaceErr := catalog.Replace(snapshot); replaceErr != nil {
			t.Fatal(replaceErr)
		}
	}
	return catalog
}

func TestSnapshotHandlerServesOneCommitStampedRepositoryView(t *testing.T) {
	recorder := httptest.NewRecorder()
	SnapshotHandler(synchronizedCatalog(t)).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/r/api/f/main.go", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	for _, want := range []string{"Payments API", "The provider snapshot is immutable.", snapshotCommit, `value="/r/web/"`, `class="panel-actions"`} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Errorf("page is missing %q", want)
		}
	}
	if got := recorder.Header().Get("X-Koment-Commit"); got != snapshotCommit {
		t.Errorf("commit header = %q", got)
	}
}

func TestSnapshotHandlerKeepsServingFailuresExplicit(t *testing.T) {
	repository := serving.Repository{
		Identity: application.RepositoryIdentity{ID: "api"}, Provider: "github", Remote: "example/api", Branch: "main", Default: true,
	}
	catalog, err := serving.NewCatalog([]serving.Repository{repository})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	SnapshotHandler(catalog).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/r/api/", nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "no synchronized snapshot") {
		t.Fatalf("want explicit 503, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestSnapshotHandlerScopesNavigationAndDefaultRedirect(t *testing.T) {
	handler := SnapshotHandlerFor(synchronizedCatalog(t), map[string]bool{"web": true})
	root := httptest.NewRecorder()
	handler.ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/", nil))
	if root.Code != http.StatusFound || root.Header().Get("Location") != "/r/web/" {
		t.Fatalf("root = %d %q", root.Code, root.Header().Get("Location"))
	}
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/r/api/", nil))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", denied.Code)
	}
}
