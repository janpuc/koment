// Package anchor decides where an annotation still applies, and says so when it
// no longer does.
package anchor

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"

	"github.com/janpuc/koment/internal/store"
)

type Status string

const (
	StatusOK       Status = "ok"
	StatusMoved    Status = "moved"
	StatusDrifted  Status = "drifted"
	StatusOrphaned Status = "orphaned"
)

// IsFailure reports whether a status means the annotation can no longer be
// trusted, and so must fail a check rather than be served.
func (s Status) IsFailure() bool {
	return s == StatusDrifted || s == StatusOrphaned
}

type Resolution struct {
	Annotation  store.Annotation
	Status      Status
	Line        int
	Occurrences int
}

// Resolve locates an annotation in the current content of its file.
func Resolve(annotation store.Annotation, content []byte) Resolution {
	if annotation.Scope == store.ScopeFile {
		return Resolution{Annotation: annotation, Status: StatusOK}
	}

	found := excerptLines(content, annotation.Excerpt)
	switch {
	case len(found) == 0:
		return Resolution{Annotation: annotation, Status: StatusDrifted}
	case slices.Contains(found, annotation.LastSeenLine):
		return Resolution{annotation, StatusOK, annotation.LastSeenLine, len(found)}
	case annotation.LastSeenLine == 0:
		return Resolution{annotation, StatusOK, found[0], len(found)}
	default:
		return Resolution{annotation, StatusMoved, found[0], len(found)}
	}
}

func ResolveOrphaned(annotation store.Annotation) Resolution {
	return Resolution{Annotation: annotation, Status: StatusOrphaned}
}

// ResolveStored loads one file's record and resolves it against the working tree.
func ResolveStored(annotations *store.Store, file string) ([]Resolution, error) {
	record, err := annotations.Load(file)
	if err != nil {
		return nil, err
	}
	sourcePath, err := annotations.SourcePath(file)
	if err != nil {
		return nil, err
	}
	return ResolveRecord(record, sourcePath)
}

// ResolveRecord resolves every annotation in a record against the file on disk.
func ResolveRecord(record *store.Record, sourcePath string) ([]Resolution, error) {
	content, err := os.ReadFile(sourcePath)
	if errors.Is(err, fs.ErrNotExist) {
		return resolveAll(record, ResolveOrphaned), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", sourcePath, err)
	}

	return resolveAll(record, func(annotation store.Annotation) Resolution {
		return Resolve(annotation, content)
	}), nil
}

func resolveAll(record *store.Record, resolve func(store.Annotation) Resolution) []Resolution {
	resolutions := make([]Resolution, len(record.Annotations))
	for i, annotation := range record.Annotations {
		resolutions[i] = resolve(annotation)
	}
	return resolutions
}

// ExcerptLines reports the 1-based lines at which an excerpt occurs verbatim.
func ExcerptLines(content []byte, excerpt string) []int {
	return excerptLines(content, excerpt)
}

func excerptLines(content []byte, excerpt string) []int {
	needle := []byte(excerpt)
	if len(needle) == 0 {
		return nil
	}

	var lines []int
	searched, line := 0, 1
	for {
		index := bytes.Index(content[searched:], needle)
		if index < 0 {
			return lines
		}
		start := searched + index
		line += bytes.Count(content[searched:start], []byte{'\n'})
		lines = append(lines, line)

		searched = start + 1
		line += bytes.Count(content[start:searched], []byte{'\n'})
	}
}
