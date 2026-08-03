// Package repository holds the set of repositories a koment deployment serves.
// Identity is assigned rather than computed, so a repository that moves keeps
// its index and its history (ADR 0024).
package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	yaml "go.yaml.in/yaml/v3"

	"github.com/janpuc/koment/internal/store"
)

const (
	EnvConfig = "KOMENT_CONFIG"
	EnvRepos  = "KOMENT_REPOS"
	EnvRepo   = "KOMENT_REPO"
)

type Repository struct {
	ID            string `yaml:"id"`
	Name          string `yaml:"name,omitempty"`
	Root          string `yaml:"root"`
	CloneURL      string `yaml:"clone_url,omitempty"`
	DefaultBranch string `yaml:"default_branch,omitempty"`
}

func (r Repository) Store() *store.Store { return store.Open(r.Root) }

func (r Repository) Display() string {
	if r.Name != "" {
		return r.Name
	}
	return r.ID
}

// Set is what koment serves. It is ordered so that listings and exports are
// deterministic rather than however a map felt like iterating.
type Set struct{ repositories []Repository }

type file struct {
	Repositories []Repository `yaml:"repositories"`
}

// Load reads the registry, preferring the richest source that is configured so
// that a laptop needs no configuration and a deployment can say more.
func Load(workingDirectory string) (*Set, error) {
	if path := os.Getenv(EnvConfig); path != "" {
		return loadFile(path)
	}
	if list := os.Getenv(EnvRepos); list != "" {
		return loadList(list)
	}
	if root := os.Getenv(EnvRepo); root != "" {
		return discover(root)
	}
	return discover(workingDirectory)
}

func loadFile(path string) (*Set, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var parsed file
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(parsed.Repositories) == 0 {
		return nil, fmt.Errorf("%s lists no repositories", path)
	}

	set := &Set{}
	for i := range parsed.Repositories {
		entry := parsed.Repositories[i]
		if entry.Root, err = absolute(entry.Root, filepath.Dir(path)); err != nil {
			return nil, err
		}
		if err := set.add(entry); err != nil {
			return nil, fmt.Errorf("in %s: %w", path, err)
		}
	}
	return set, nil
}

// loadList parses KOMENT_REPOS, which is name=path pairs. It is the small case:
// a container with a few mounts and nothing else worth saying.
func loadList(list string) (*Set, error) {
	set := &Set{}
	for entry := range strings.SplitSeq(list, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		id, root, found := strings.Cut(entry, "=")
		if !found {
			return nil, fmt.Errorf("%s entry %q must look like name=/path", EnvRepos, entry)
		}
		absoluteRoot, err := absolute(strings.TrimSpace(root), "")
		if err != nil {
			return nil, err
		}
		if err := set.add(Repository{ID: strings.TrimSpace(id), Root: absoluteRoot}); err != nil {
			return nil, fmt.Errorf("in %s: %w", EnvRepos, err)
		}
	}
	if len(set.repositories) == 0 {
		return nil, fmt.Errorf("%s is set but lists no repositories", EnvRepos)
	}
	return set, nil
}

// discover is the single-repository case, unchanged from before there was a
// registry: walk up for a store and name it after its directory.
func discover(start string) (*Set, error) {
	root, err := store.FindRoot(start)
	if err != nil {
		return nil, err
	}

	set := &Set{}
	return set, set.add(Repository{ID: identifier(root), Root: root})
}

// identifier names an unconfigured repository after its directory, falling back
// to a hash only when that is not usable as an id. A configured repository
// never reaches here — its id is assigned (ADR 0024).
func identifier(root string) string {
	base := strings.ToLower(filepath.Base(root))
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		case r == ' ' || r == '.':
			return '-'
		}
		return -1
	}, base)

	if cleaned == "" || cleaned == "-" {
		sum := sha256.Sum256([]byte(root))
		return hex.EncodeToString(sum[:])[:16]
	}
	return cleaned
}

func (s *Set) add(entry Repository) error {
	switch {
	case entry.ID == "":
		return fmt.Errorf("repository at %s has no id", entry.Root)
	case entry.Root == "":
		return fmt.Errorf("repository %s has no root", entry.ID)
	}
	if _, taken := s.ByID(entry.ID); taken {
		return fmt.Errorf("duplicate repository id %s", entry.ID)
	}
	s.repositories = append(s.repositories, entry)
	return nil
}

func (s *Set) All() []Repository {
	ordered := append([]Repository(nil), s.repositories...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	return ordered
}

func (s *Set) Len() int { return len(s.repositories) }

func (s *Set) ByID(id string) (Repository, bool) {
	for _, entry := range s.repositories {
		if entry.ID == id {
			return entry, true
		}
	}
	return Repository{}, false
}

// Resolve accepts an id or a display name, because an agent reading a listing
// may reasonably send either back.
func (s *Set) Resolve(reference string) (Repository, bool) {
	if entry, found := s.ByID(reference); found {
		return entry, true
	}
	for _, entry := range s.repositories {
		if entry.Name != "" && strings.EqualFold(entry.Name, reference) {
			return entry, true
		}
	}
	return Repository{}, false
}

// Only returns the repository when there is exactly one, which is how the
// single-repository case keeps working without anyone naming it.
func (s *Set) Only() (Repository, bool) {
	if len(s.repositories) == 1 {
		return s.repositories[0], true
	}
	return Repository{}, false
}

// IDs is for error messages that have to tell a caller what it could have said.
func (s *Set) IDs() []string {
	ids := make([]string, 0, len(s.repositories))
	for _, entry := range s.All() {
		ids = append(ids, entry.ID)
	}
	return ids
}

func absolute(path, relativeTo string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("repository root is empty")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	if relativeTo != "" {
		return filepath.Clean(filepath.Join(relativeTo, path)), nil
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", path, err)
	}
	return resolved, nil
}
