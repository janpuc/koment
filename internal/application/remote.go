package application

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/store"
)

func DraftAnnotation(snapshot *RepositorySnapshot, input AddInput) (store.Annotation, error) {
	if snapshot == nil {
		return store.Annotation{}, errors.New("cannot write without a repository snapshot")
	}
	if snapshot.Commit == "" {
		return store.Annotation{}, errors.New("cannot write from a snapshot without a commit")
	}
	file, found := snapshot.File(input.File)
	if !found || !file.Exists {
		return store.Annotation{}, fmt.Errorf("source file %s is not present in commit %s", input.File, snapshot.Commit)
	}
	if err := input.Author.Validate(); err != nil {
		return store.Annotation{}, err
	}
	if _, err := store.ParseKind(string(input.Kind)); err != nil {
		return store.Annotation{}, err
	}
	id, err := store.NewID(time.Now())
	if err != nil {
		return store.Annotation{}, err
	}
	record := store.Annotation{
		Version: store.RecordVersion, ID: id, File: file.Path, Kind: input.Kind,
		Body: store.WrapProse(input.Body), Created: store.Today(), Author: input.Author, Policy: input.Policy,
	}
	if input.Excerpt == "" {
		record.Anchor = store.Anchor{Scope: store.ScopeFile}
	} else {
		captured, captureErr := anchor.Capture(file.Content, input.Excerpt)
		if captureErr != nil {
			lines := anchor.ExcerptLines(file.Content, input.Excerpt)
			if len(lines) == 0 {
				return store.Annotation{}, fmt.Errorf("excerpt not found in %s; it must match commit %s verbatim", file.Path, snapshot.Commit)
			}
			return store.Annotation{}, fmt.Errorf("excerpt matches %d places in %s (lines %v); extend it until it is unique", len(lines), file.Path, lines)
		}
		record.Anchor = captured
	}
	line := record.Anchor.LastSeenLine
	endLine := line
	if line > 0 {
		endLine += strings.Count(strings.TrimSuffix(input.Excerpt, "\n"), "\n")
	}
	record.Git = &store.GitContext{Commit: snapshot.Commit, Path: file.Path, Line: line, EndLine: endLine}
	if err := record.Validate(); err != nil {
		return store.Annotation{}, err
	}
	return record, nil
}
