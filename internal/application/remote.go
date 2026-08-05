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
	if _, err := store.ParseType(string(input.Kind)); err != nil {
		return store.Annotation{}, err
	}
	id, err := store.NewID(time.Now())
	if err != nil {
		return store.Annotation{}, err
	}
	record := store.Annotation{
		APIVersion: store.APIVersion,
		Kind:       store.KindAnnotation,
		Metadata:   store.Metadata{ID: id, Created: store.Now()},
		Spec: store.Spec{
			Target: store.Target{File: file.Path},
			Type:   input.Kind,
			Body:   store.WrapProse(input.Body),
			Anchor: store.Anchor{Scope: store.ScopeFile},
			Author: input.Author,
			Policy: input.Policy,
		},
	}
	if input.Excerpt != "" {
		captured, line, captureErr := anchor.Capture(file.Content, input.Excerpt)
		if captureErr != nil {
			lines := anchor.ExcerptLines(file.Content, input.Excerpt)
			if len(lines) == 0 {
				return store.Annotation{}, fmt.Errorf("excerpt not found in %s; it must match commit %s verbatim", file.Path, snapshot.Commit)
			}
			return store.Annotation{}, fmt.Errorf("excerpt matches %d places in %s (lines %v); extend it until it is unique", len(lines), file.Path, lines)
		}
		record.Spec.Anchor = captured
		record.Status.LastSeenLine = line
	}
	line := record.Status.LastSeenLine
	endLine := line
	if line > 0 {
		endLine += strings.Count(strings.TrimSuffix(input.Excerpt, "\n"), "\n")
	}
	record.Spec.Git = &store.GitContext{Commit: snapshot.Commit, Path: file.Path, Line: line, EndLine: endLine}
	record.Status.Observe(store.AnchorOK, snapshot.Commit, store.Now())
	if err := record.Validate(); err != nil {
		return store.Annotation{}, err
	}
	return record, nil
}
