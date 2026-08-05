package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/auth"
	"github.com/janpuc/koment/internal/metrics"
	"github.com/janpuc/koment/internal/serving"
	"github.com/janpuc/koment/internal/store"
)

type testMaterializer struct {
	record store.Annotation
}

func (materializer *testMaterializer) Materialize(
	_ context.Context, _ serving.Repository, _ string, record store.Annotation,
) (serving.Materialization, error) {
	materializer.record = record
	return serving.Materialization{
		Branch: "koment/" + record.Metadata.ID, Commit: testCommit, PullRequest: 42,
		URL: "https://github.com/example/api/pull/42",
	}, nil
}

const testCommit = "0123456789abcdef0123456789abcdef01234567"

func testCatalog(t *testing.T, synchronized bool) *serving.Catalog {
	t.Helper()
	repository := serving.Repository{
		Identity: application.RepositoryIdentity{ID: "api", Name: "Payments API"},
		Provider: "github", Remote: "example/api", Branch: "main", Default: true,
	}
	catalog, err := serving.NewCatalog([]serving.Repository{repository})
	if err != nil {
		t.Fatal(err)
	}
	if !synchronized {
		return catalog
	}
	record := store.Annotation{
		APIVersion: store.APIVersion,
		Kind:       store.KindAnnotation,
		Metadata:   store.Metadata{ID: "01JQ8ZK3M4N5P6R7S8T9V0W1X2", Created: store.Timestamp{Time: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)}},
		Spec: store.Spec{
			Target: store.Target{File: "main.go"},
			Type:   store.TypeWhy,
			Body:   "The service owns synchronization.",
			Anchor: store.Anchor{Scope: store.ScopeExcerpt, Excerpt: "serve()"},
			Author: store.Author{Name: "Agent", Kind: store.AuthorAgent, Source: store.FromExplicit},
		},
		Status: store.Status{LastSeenLine: 3},
	}
	snapshot, err := application.AssembleSnapshot(application.SnapshotInput{
		Repository: repository.Identity, Commit: testCommit, Records: []store.Annotation{record},
		Sources: map[string][]byte{"main.go": []byte("package main\n\nserve()\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.Replace(snapshot); err != nil {
		t.Fatal(err)
	}
	return catalog
}

func loopbackAuthenticator(t *testing.T) *auth.Authenticator {
	t.Helper()
	authenticator, err := auth.New(auth.Configuration{AllowLoopback: true})
	if err != nil {
		t.Fatal(err)
	}
	return authenticator
}

func TestHealthRoutesArePublicAndReadinessReflectsSnapshots(t *testing.T) {
	for _, fixture := range []struct {
		synchronized bool
		path         string
		status       int
	}{{false, "/livez", http.StatusOK}, {false, "/readyz", http.StatusServiceUnavailable}, {true, "/readyz", http.StatusOK}} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, fixture.path, nil)
		request.RemoteAddr = "198.51.100.1:1234"
		Handler(testCatalog(t, fixture.synchronized), loopbackAuthenticator(t), metrics.Discard{}, nil).ServeHTTP(recorder, request)
		if recorder.Code != fixture.status {
			t.Errorf("%s synchronized=%v: want %d, got %d: %s", fixture.path, fixture.synchronized, fixture.status, recorder.Code, recorder.Body.String())
		}
	}
}

func TestSourceRoutesRequireAuthenticationAndCarrySecurityHeaders(t *testing.T) {
	handler := Handler(testCatalog(t, true), loopbackAuthenticator(t), metrics.Discard{}, nil)
	unauthorized := httptest.NewRecorder()
	remote := httptest.NewRequest(http.MethodGet, "/r/api/", nil)
	remote.RemoteAddr = "198.51.100.1:1234"
	handler.ServeHTTP(unauthorized, remote)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", unauthorized.Code)
	}
	if unauthorized.Header().Get("WWW-Authenticate") == "" {
		t.Error("authentication challenge is missing")
	}

	authorized := httptest.NewRecorder()
	local := httptest.NewRequest(http.MethodGet, "/r/api/", nil)
	local.RemoteAddr = "127.0.0.1:1234"
	handler.ServeHTTP(authorized, local)
	if authorized.Code != http.StatusOK || !strings.Contains(authorized.Body.String(), "Payments API") {
		t.Fatalf("want authenticated page, got %d: %s", authorized.Code, authorized.Body.String())
	}
	for _, header := range []string{"Content-Security-Policy", "Referrer-Policy", "X-Content-Type-Options", "X-Frame-Options"} {
		if authorized.Header().Get(header) == "" {
			t.Errorf("%s is missing", header)
		}
	}
}

func TestRepositoryConfigurationIsStrictAndBuildsServingIdentity(t *testing.T) {
	directory := t.TempDir()
	valid := filepath.Join(directory, "repositories.yaml")
	if err := os.WriteFile(valid, []byte("repositories:\n  - id: api\n    name: Payments API\n    provider: github\n    remote: example/api\n    default_branch: main\n    default: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	repositories, err := loadRepositories(valid)
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 1 || repositories[0].Identity.ID != "api" || repositories[0].Remote != "example/api" {
		t.Fatalf("repositories = %#v", repositories)
	}
	invalid := filepath.Join(directory, "invalid.yaml")
	if err := os.WriteFile(invalid, []byte("repositories: []\nunknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRepositories(invalid); err == nil {
		t.Fatal("unknown configuration key was accepted")
	}
}

func TestAuthenticatedHumanWriteMaterializesAndRedirectsToReview(t *testing.T) {
	materializer := &testMaterializer{}
	handler := Handler(testCatalog(t, true), loopbackAuthenticator(t), metrics.Discard{}, materializer)

	page := httptest.NewRecorder()
	pageRequest := httptest.NewRequest(http.MethodGet, "http://koment.local/r/api/f/main.go", nil)
	pageRequest.RemoteAddr = "127.0.0.1:1234"
	handler.ServeHTTP(page, pageRequest)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Add rationale") {
		t.Fatalf("write form = %d: %s", page.Code, page.Body.String())
	}

	form := url.Values{
		"file": []string{"main.go"}, "excerpt": []string{"serve()"},
		"kind": []string{"why"}, "body": []string{"The service owns process lifetime."},
	}
	created := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "http://koment.local/r/api/annotations", strings.NewReader(form.Encode()))
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://koment.local")
	handler.ServeHTTP(created, request)
	if created.Code != http.StatusSeeOther {
		t.Fatalf("want 303, got %d: %s", created.Code, created.Body.String())
	}
	location := created.Header().Get("Location")
	if !strings.Contains(location, "created=") || !strings.Contains(location, "review=https") {
		t.Fatalf("location = %q", location)
	}
	if materializer.record.Spec.Author.Verified != "loopback" || materializer.record.Spec.Git.Commit != testCommit {
		t.Fatalf("record = %#v", materializer.record)
	}
}
