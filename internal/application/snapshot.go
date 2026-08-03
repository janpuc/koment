package application

import (
	"errors"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/provenance"
	"github.com/janpuc/koment/internal/repository"
	"github.com/janpuc/koment/internal/store"
)

// RepositorySnapshot is one internally consistent read of a repository.
type RepositorySnapshot struct {
	Repository  RepositoryIdentity
	Commit      string
	Dirty       bool
	GeneratedAt time.Time
	Files       []FileSnapshot
}

// RepositoryIdentity is stable presentation metadata for a snapshot.
type RepositoryIdentity struct {
	ID            string
	Name          string
	CloneURL      string
	DefaultBranch string
}

// FileSnapshot contains source and resolved annotations from the same read.
type FileSnapshot struct {
	Path        string
	Content     []byte
	Exists      bool
	Annotations []AnnotationView
}

// AnnotationView is the common record and resolution presented by every reader.
type AnnotationView struct {
	Record      store.Annotation
	Status      anchor.Status
	Line        int
	Occurrences int
	Warning     string
}

// BuildSnapshot reads every annotation and annotated source file once.
func BuildSnapshot(entry repository.Repository) (*RepositorySnapshot, error) {
	annotations := entry.Store()
	records, err := annotations.All()
	if err != nil {
		return nil, err
	}
	sort.Slice(records, func(left, right int) bool { return records[left].ID < records[right].ID })

	grouped := make(map[string][]store.Annotation)
	for _, record := range records {
		grouped[record.File] = append(grouped[record.File], record)
	}
	paths := make([]string, 0, len(grouped))
	for path := range grouped {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	snapshot := &RepositorySnapshot{
		GeneratedAt: time.Now().UTC(),
		Repository: RepositoryIdentity{
			ID: entry.ID, Name: entry.Display(), CloneURL: entry.CloneURL,
			DefaultBranch: entry.DefaultBranch,
		},
	}
	if commit, commitErr := provenance.HeadCommit(entry.Root); commitErr == nil {
		snapshot.Commit = commit
		snapshot.Dirty = provenance.TreeIsDirty(entry.Root)
	} else if !errors.Is(commitErr, provenance.ErrNoGit) {
		return nil, commitErr
	}

	for _, path := range paths {
		file := FileSnapshot{Path: path, Exists: true}
		file.Content, err = annotations.ReadSource(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			file.Exists = false
			for _, record := range grouped[path] {
				file.Annotations = append(file.Annotations, describe(anchor.ResolveOrphaned(record)))
			}
		case err != nil:
			return nil, err
		default:
			for _, record := range grouped[path] {
				file.Annotations = append(file.Annotations, describe(anchor.Resolve(record, file.Content)))
			}
		}
		snapshot.Files = append(snapshot.Files, file)
	}
	return snapshot, nil
}

func describe(resolution anchor.Resolution) AnnotationView {
	return AnnotationView{
		Record: resolution.Annotation, Status: resolution.Status, Line: resolution.Line,
		Occurrences: resolution.Occurrences, Warning: WarningFor(resolution.Status),
	}
}

// WarningFor is the single stale-record warning policy for every surface.
func WarningFor(status anchor.Status) string {
	switch status {
	case anchor.StatusAmbiguous:
		return "STALE: the excerpt matches several places and its context does not identify one. Treat it as history until someone explicitly reanchors it."
	case anchor.StatusDrifted:
		return "STALE: the annotated code changed. Treat this as history until someone explicitly reanchors it."
	case anchor.StatusOrphaned:
		return "STALE: the annotated file no longer exists. Treat this as history only."
	default:
		return ""
	}
}

// File returns one annotated file from the snapshot.
func (s *RepositorySnapshot) File(path string) (FileSnapshot, bool) {
	for _, file := range s.Files {
		if file.Path == path {
			return file, true
		}
	}
	return FileSnapshot{}, false
}

// Search returns annotations whose record fields contain the query.
func (s *RepositorySnapshot) Search(query string) []AnnotationView {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil
	}
	var matches []AnnotationView
	for _, file := range s.Files {
		for _, annotation := range file.Annotations {
			record := annotation.Record
			haystack := strings.ToLower(strings.Join([]string{
				record.ID, record.File, string(record.Kind), record.Body,
				record.Author.Name, record.Author.Email, record.Author.Account,
			}, "\n"))
			if strings.Contains(haystack, needle) {
				matches = append(matches, annotation)
			}
		}
	}
	return matches
}

// Counts returns resolution counts for the repository.
func (s *RepositorySnapshot) Counts() map[anchor.Status]int {
	counts := map[anchor.Status]int{}
	for _, file := range s.Files {
		for _, annotation := range file.Annotations {
			counts[annotation.Status]++
		}
	}
	return counts
}
