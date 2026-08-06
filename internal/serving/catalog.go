package serving

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/koment-dev/koment/internal/application"
	"github.com/koment-dev/koment/internal/store"
)

// Repository identifies one remote repository served by koment.
type Repository struct {
	Identity application.RepositoryIdentity
	Provider string
	Remote   string
	Branch   string
	Default  bool
}

func (r Repository) Validate() error {
	switch {
	case strings.TrimSpace(r.Identity.ID) == "":
		return errors.New("repository has no id")
	case r.Provider != "github":
		return fmt.Errorf("repository %s has unsupported provider %q", r.Identity.ID, r.Provider)
	case !validGitHubRemote(r.Remote):
		return fmt.Errorf("repository %s remote %q must look like owner/name", r.Identity.ID, r.Remote)
	case strings.TrimSpace(r.Branch) == "":
		return fmt.Errorf("repository %s has no branch", r.Identity.ID)
	}
	return nil
}

func validGitHubRemote(remote string) bool {
	owner, name, found := strings.Cut(remote, "/")
	return found && owner != "" && name != "" && !strings.Contains(name, "/") &&
		owner != "." && owner != ".." && name != "." && name != ".."
}

// State is the current immutable snapshot and synchronization status.
type State struct {
	Repository  Repository
	Snapshot    *application.RepositorySnapshot
	LastAttempt time.Time
	LastSuccess time.Time
	Failure     string
}

// Catalog owns the active snapshot for every configured repository.
type Catalog struct {
	mu           sync.RWMutex
	repositories map[string]Repository
	states       map[string]State
	defaultID    string
	now          func() time.Time
}

func NewCatalog(repositories []Repository) (*Catalog, error) {
	catalog := &Catalog{
		repositories: make(map[string]Repository, len(repositories)),
		states:       make(map[string]State, len(repositories)),
		now:          func() time.Time { return time.Now().UTC() },
	}
	for _, repository := range repositories {
		if err := repository.Validate(); err != nil {
			return nil, err
		}
		id := repository.Identity.ID
		if _, duplicate := catalog.repositories[id]; duplicate {
			return nil, fmt.Errorf("duplicate repository id %s", id)
		}
		if repository.Default {
			if catalog.defaultID != "" {
				return nil, fmt.Errorf("repositories %s and %s are both default", catalog.defaultID, id)
			}
			catalog.defaultID = id
		}
		catalog.repositories[id] = repository
		catalog.states[id] = State{Repository: repository}
	}
	if len(repositories) == 0 {
		return nil, errors.New("served repository set is empty")
	}
	if catalog.defaultID == "" {
		return nil, errors.New("served repository set has no default")
	}
	return catalog, nil
}

func (c *Catalog) Replace(snapshot *application.RepositorySnapshot) error {
	if snapshot == nil {
		return errors.New("cannot activate a nil snapshot")
	}
	id := snapshot.Repository.ID
	c.mu.Lock()
	defer c.mu.Unlock()
	repository, configured := c.repositories[id]
	if !configured {
		return fmt.Errorf("repository %s is not configured", id)
	}
	if snapshot.Commit == "" {
		return fmt.Errorf("repository %s snapshot has no commit", id)
	}
	now := c.now()
	c.states[id] = State{
		Repository: repository, Snapshot: cloneSnapshot(snapshot),
		LastAttempt: now, LastSuccess: now,
	}
	return nil
}

func (c *Catalog) RecordFailure(id string, failure error) error {
	if failure == nil {
		return errors.New("cannot record a nil synchronization failure")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	state, configured := c.states[id]
	if !configured {
		return fmt.Errorf("repository %s is not configured", id)
	}
	state.LastAttempt = c.now()
	state.Failure = failure.Error()
	c.states[id] = state
	return nil
}

func (c *Catalog) State(id string) (State, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, found := c.states[id]
	return cloneState(state), found
}

func (c *Catalog) Default() (State, bool) {
	return c.State(c.defaultID)
}

func (c *Catalog) Repositories() []Repository {
	c.mu.RLock()
	defer c.mu.RUnlock()
	configured := make([]Repository, 0, len(c.repositories))
	for _, repository := range c.repositories {
		configured = append(configured, repository)
	}
	sort.Slice(configured, func(left, right int) bool {
		return configured[left].Identity.ID < configured[right].Identity.ID
	})
	return configured
}

func (c *Catalog) IDs() []string {
	repositories := c.Repositories()
	ids := make([]string, len(repositories))
	for index, repository := range repositories {
		ids[index] = repository.Identity.ID
	}
	return ids
}

func (c *Catalog) States() []State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	states := make([]State, 0, len(c.states))
	for _, state := range c.states {
		states = append(states, cloneState(state))
	}
	sort.Slice(states, func(left, right int) bool {
		return states[left].Repository.Identity.ID < states[right].Repository.Identity.ID
	})
	return states
}

func (c *Catalog) Ready() error {
	var failures []error
	for _, state := range c.States() {
		switch {
		case state.Snapshot == nil:
			failures = append(failures, fmt.Errorf("repository %s has no snapshot", state.Repository.Identity.ID))
		case state.Failure != "":
			failures = append(failures, fmt.Errorf("repository %s refresh failed: %s", state.Repository.Identity.ID, state.Failure))
		}
	}
	return errors.Join(failures...)
}

func cloneState(state State) State {
	state.Snapshot = cloneSnapshot(state.Snapshot)
	return state
}

func cloneSnapshot(snapshot *application.RepositorySnapshot) *application.RepositorySnapshot {
	if snapshot == nil {
		return nil
	}
	cloned := *snapshot
	cloned.Files = make([]application.FileSnapshot, len(snapshot.Files))
	for fileIndex, file := range snapshot.Files {
		cloned.Files[fileIndex] = file
		cloned.Files[fileIndex].Content = append([]byte(nil), file.Content...)
		cloned.Files[fileIndex].Annotations = append([]application.AnnotationView(nil), file.Annotations...)
		for annotationIndex := range cloned.Files[fileIndex].Annotations {
			cloned.Files[fileIndex].Annotations[annotationIndex].Record = cloneRecord(
				cloned.Files[fileIndex].Annotations[annotationIndex].Record,
			)
		}
	}
	return &cloned
}

func cloneRecord(record store.Annotation) store.Annotation {
	if record.Spec.Git != nil {
		gitContext := *record.Spec.Git
		record.Spec.Git = &gitContext
	}
	if record.Spec.Policy != nil {
		policy := *record.Spec.Policy
		record.Spec.Policy = &policy
	}
	return record
}
