package serving

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koment-dev/koment/internal/application"
)

func configuredRepository(id string, primary bool) Repository {
	return Repository{
		Identity: application.RepositoryIdentity{ID: id, Name: strings.ToUpper(id), DefaultBranch: "main"},
		Provider: "github", Remote: "example/" + id, Branch: "main", Default: primary,
	}
}

func providerSnapshot(repository Repository, commit string) *application.RepositorySnapshot {
	return &application.RepositorySnapshot{
		Repository: repository.Identity, Commit: commit,
		Files: []application.FileSnapshot{{Path: "main.go", Exists: true, Content: []byte(commit)}},
	}
}

func TestCatalogRequiresOneDefaultRepository(t *testing.T) {
	if _, err := NewCatalog([]Repository{configuredRepository("one", false)}); err == nil {
		t.Fatal("want a missing-default error")
	}
	if _, err := NewCatalog([]Repository{configuredRepository("one", true), configuredRepository("two", true)}); err == nil {
		t.Fatal("want a duplicate-default error")
	}
}

func TestCatalogKeepsThePreviousSnapshotAfterARefreshFailure(t *testing.T) {
	repository := configuredRepository("one", true)
	catalog, err := NewCatalog([]Repository{repository})
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	catalog.now = func() time.Time { return clock }
	if err := catalog.Replace(providerSnapshot(repository, strings.Repeat("a", 40))); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Minute)
	if err := catalog.RecordFailure("one", errors.New("provider unavailable")); err != nil {
		t.Fatal(err)
	}
	state, found := catalog.Default()
	if !found || state.Snapshot.Commit != strings.Repeat("a", 40) {
		t.Fatalf("state = %#v", state)
	}
	if state.Failure != "provider unavailable" || !state.LastAttempt.Equal(clock) || state.LastSuccess.Equal(clock) {
		t.Fatalf("failure metadata = %#v", state)
	}
	if err := catalog.Ready(); err == nil || !strings.Contains(err.Error(), "provider unavailable") {
		t.Fatalf("ready error = %v", err)
	}
}

func TestCatalogReplacementIsAtomicForConcurrentReaders(t *testing.T) {
	repository := configuredRepository("one", true)
	catalog, err := NewCatalog([]Repository{repository})
	if err != nil {
		t.Fatal(err)
	}
	first := strings.Repeat("a", 40)
	second := strings.Repeat("b", 40)
	if err := catalog.Replace(providerSnapshot(repository, first)); err != nil {
		t.Fatal(err)
	}

	var wait sync.WaitGroup
	for range 16 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range 500 {
				state, found := catalog.State("one")
				if !found || state.Snapshot == nil {
					t.Error("snapshot disappeared")
					return
				}
				commit := state.Snapshot.Commit
				content := string(state.Snapshot.Files[0].Content)
				if commit != content || commit != first && commit != second {
					t.Errorf("mixed snapshot commit=%q content=%q", commit, content)
					return
				}
			}
		}()
	}
	if err := catalog.Replace(providerSnapshot(repository, second)); err != nil {
		t.Fatal(err)
	}
	wait.Wait()
}

type sourceFunc func(context.Context, Repository) (*application.RepositorySnapshot, error)

func (function sourceFunc) Snapshot(ctx context.Context, repository Repository) (*application.RepositorySnapshot, error) {
	return function(ctx, repository)
}

func TestSynchronizerDoesNotReplaceAValidSnapshotWithAnError(t *testing.T) {
	repository := configuredRepository("one", true)
	catalog, err := NewCatalog([]Repository{repository})
	if err != nil {
		t.Fatal(err)
	}
	synchronizer := Synchronizer{Catalog: catalog, Source: sourceFunc(func(context.Context, Repository) (*application.RepositorySnapshot, error) {
		return providerSnapshot(repository, strings.Repeat("a", 40)), nil
	})}
	if err := synchronizer.RefreshAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	synchronizer.Source = sourceFunc(func(context.Context, Repository) (*application.RepositorySnapshot, error) {
		return nil, errors.New("rate limited")
	})
	if err := synchronizer.RefreshAll(context.Background()); err == nil {
		t.Fatal("want the refresh failure")
	}
	state, _ := catalog.State("one")
	if state.Snapshot.Commit != strings.Repeat("a", 40) || state.Failure != "rate limited" {
		t.Fatalf("state = %#v", state)
	}
}
