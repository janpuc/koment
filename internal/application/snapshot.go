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

// SnapshotInput is a complete repository read from one source revision.
type SnapshotInput struct {
	Repository  RepositoryIdentity
	Commit      string
	Dirty       bool
	GeneratedAt time.Time
	Records     []store.Annotation
	Sources     map[string][]byte
}

// BuildSnapshot reads every annotation and annotated source file once.
func BuildSnapshot(entry repository.Repository) (*RepositorySnapshot, error) {
	annotations := entry.Store()
	records, err := annotations.All()
	if err != nil {
		return nil, err
	}
	sources := make(map[string][]byte)
	for _, record := range records {
		if _, loaded := sources[record.Spec.Target.File]; loaded {
			continue
		}
		content, readErr := annotations.ReadSource(record.Spec.Target.File)
		switch {
		case errors.Is(readErr, fs.ErrNotExist):
		case readErr != nil:
			return nil, readErr
		default:
			sources[record.Spec.Target.File] = content
		}
	}

	input := SnapshotInput{
		Repository: RepositoryIdentity{
			ID: entry.ID, Name: entry.Display(), CloneURL: entry.CloneURL,
			DefaultBranch: entry.DefaultBranch,
		},
		GeneratedAt: time.Now().UTC(), Records: records, Sources: sources,
	}
	if commit, commitErr := provenance.HeadCommit(entry.Root); commitErr == nil {
		input.Commit = commit
		input.Dirty = provenance.TreeIsDirty(entry.Root)
	} else if !errors.Is(commitErr, provenance.ErrNoGit) {
		return nil, commitErr
	}
	return AssembleSnapshot(input)
}

// AssembleSnapshot resolves a complete source revision without performing I/O.
func AssembleSnapshot(input SnapshotInput) (*RepositorySnapshot, error) {
	records := append([]store.Annotation(nil), input.Records...)
	sort.Slice(records, func(left, right int) bool { return records[left].Metadata.ID < records[right].Metadata.ID })
	grouped := make(map[string][]store.Annotation)
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return nil, err
		}
		if _, duplicate := seen[record.Metadata.ID]; duplicate {
			return nil, errors.New("duplicate annotation id " + record.Metadata.ID)
		}
		seen[record.Metadata.ID] = struct{}{}
		grouped[record.Spec.Target.File] = append(grouped[record.Spec.Target.File], record)
	}
	paths := make([]string, 0, len(grouped))
	for path := range grouped {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	generatedAt := input.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	snapshot := &RepositorySnapshot{
		Repository: input.Repository, Commit: input.Commit, Dirty: input.Dirty,
		GeneratedAt: generatedAt,
	}

	for _, path := range paths {
		content, exists := input.Sources[path]
		file := FileSnapshot{Path: path, Exists: exists}
		if !exists {
			for _, record := range grouped[path] {
				file.Annotations = append(file.Annotations, describe(anchor.ResolveOrphaned(record)))
			}
		} else {
			file.Content = append([]byte(nil), content...)
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
				record.Metadata.ID, record.Spec.Target.File, string(record.Spec.Type), record.Spec.Body,
				record.Spec.Author.Name, record.Spec.Author.Email, record.Spec.Author.Account,
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
