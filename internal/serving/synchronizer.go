package serving

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/janpuc/koment/internal/application"
)

// SnapshotSource builds one immutable snapshot from a provider commit.
type SnapshotSource interface {
	Snapshot(context.Context, Repository) (*application.RepositorySnapshot, error)
}

// Synchronizer refreshes configured repositories outside request handling.
type Synchronizer struct {
	Catalog *Catalog
	Source  SnapshotSource
}

func (s Synchronizer) Refresh(ctx context.Context, repository Repository) error {
	if s.Catalog == nil || s.Source == nil {
		return errors.New("synchronizer needs a catalog and snapshot source")
	}
	snapshot, err := s.Source.Snapshot(ctx, repository)
	if err != nil {
		if recordErr := s.Catalog.RecordFailure(repository.Identity.ID, err); recordErr != nil {
			return errors.Join(err, recordErr)
		}
		return err
	}
	if snapshot.Repository.ID != repository.Identity.ID {
		err = fmt.Errorf("provider returned repository %s for %s", snapshot.Repository.ID, repository.Identity.ID)
		if recordErr := s.Catalog.RecordFailure(repository.Identity.ID, err); recordErr != nil {
			return errors.Join(err, recordErr)
		}
		return err
	}
	return s.Catalog.Replace(snapshot)
}

func (s Synchronizer) RefreshAll(ctx context.Context) error {
	if s.Catalog == nil {
		return errors.New("synchronizer needs a catalog")
	}
	var wait sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	for _, repository := range s.Catalog.Repositories() {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := s.Refresh(ctx, repository); err != nil {
				mu.Lock()
				failures = append(failures, fmt.Errorf("refreshing %s: %w", repository.Identity.ID, err))
				mu.Unlock()
			}
		}()
	}
	wait.Wait()
	return errors.Join(failures...)
}
