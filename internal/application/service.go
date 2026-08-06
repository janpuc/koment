package application

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/koment-dev/koment/internal/anchor"
	"github.com/koment-dev/koment/internal/provenance"
	"github.com/koment-dev/koment/internal/repository"
	"github.com/koment-dev/koment/internal/store"
)

// Service owns local repository reads and mutations.
type Service struct {
	repository repository.Repository
	store      *store.Store
}

// AddInput is the complete intent needed to create an annotation.
type AddInput struct {
	File    string
	Excerpt string
	Kind    store.Type
	Title   string
	Body    string
	Author  store.Author
	Policy  *store.Policy
}

// ReanchorInput moves an existing annotation without changing its identity.
type ReanchorInput struct {
	ID      string
	File    string
	Excerpt string
}

// Mutation is the durable record and its repository-relative path.
type Mutation struct {
	Record   store.Annotation
	Path     string
	Warnings []string
}

// NewService constructs the application service for one repository.
func NewService(entry repository.Repository) *Service {
	return &Service{repository: entry, store: entry.Store()}
}

// Snapshot reads the repository through the shared snapshot contract.
func (s *Service) Snapshot() (*RepositorySnapshot, error) {
	return BuildSnapshot(s.repository)
}

// Add creates one fully validated annotation record.
func (s *Service) Add(input AddInput) (Mutation, error) {
	file, err := s.store.FromRoot(input.File)
	if err != nil {
		return Mutation{}, err
	}
	if err := input.Author.Validate(); err != nil {
		return Mutation{}, err
	}
	if _, err := store.ParseType(string(input.Kind)); err != nil {
		return Mutation{}, err
	}
	id, err := store.NewID(time.Now())
	if err != nil {
		return Mutation{}, err
	}
	record := store.Annotation{
		APIVersion: store.APIVersion,
		Kind:       store.KindAnnotation,
		Metadata:   store.Metadata{ID: id, Created: store.Now()},
		Spec: store.Spec{
			Target: store.Target{File: file},
			Type:   input.Kind,
			Title:  strings.TrimSpace(input.Title),
			Body:   store.WrapProse(input.Body),
			Author: input.Author,
			Policy: input.Policy,
		},
	}
	if err := s.anchor(&record, file, input.Excerpt); err != nil {
		return Mutation{}, err
	}
	warnings := s.captureGit(&record)
	if strings.TrimSpace(record.Spec.Title) == "" {
		warnings = append(warnings, "no title provided; the first sentence of the body will be shown as the headline (ADR 0115)")
	}
	s.observe(&record)
	if err := s.store.Save(&record); err != nil {
		return Mutation{}, err
	}
	return Mutation{Record: record, Path: recordPath(id), Warnings: warnings}, nil
}

// Reanchor changes only an annotation's target.
func (s *Service) Reanchor(input ReanchorInput) (Mutation, error) {
	record, err := s.store.FindByID(input.ID)
	if err != nil {
		return Mutation{}, err
	}
	file := record.Spec.Target.File
	if input.File != "" {
		if file, err = s.store.FromRoot(input.File); err != nil {
			return Mutation{}, err
		}
	}
	excerpt := input.Excerpt
	if excerpt == "" && record.Spec.Anchor.Scope == store.ScopeExcerpt {
		excerpt = record.Spec.Anchor.Excerpt
	}
	moved := *record
	if err := s.anchor(&moved, file, excerpt); err != nil {
		return Mutation{}, err
	}
	moved.Spec.Target.File = file
	s.observe(&moved)
	if err := s.store.Save(&moved); err != nil {
		return Mutation{}, err
	}
	return Mutation{Record: moved, Path: recordPath(moved.Metadata.ID)}, nil
}

func (s *Service) anchor(record *store.Annotation, file, excerpt string) error {
	content, err := s.store.ReadSource(file)
	if err != nil {
		return fmt.Errorf("reading %s: %w", file, err)
	}
	if excerpt == "" {
		record.Spec.Anchor = store.Anchor{Scope: store.ScopeFile}
		record.Status.LastSeenLine = 0
		return nil
	}
	captured, line, err := anchor.Capture(content, excerpt)
	if err != nil {
		lines := anchor.ExcerptLines(content, excerpt)
		switch len(lines) {
		case 0:
			return fmt.Errorf("excerpt not found in %s; it must match the file verbatim", file)
		default:
			return fmt.Errorf("excerpt matches %d places in %s (lines %v); extend it until it is unique", len(lines), file, lines)
		}
	}
	record.Spec.Anchor = captured
	record.Status.LastSeenLine = line
	return nil
}

func (s *Service) observe(record *store.Annotation) {
	commit, err := provenance.Head(s.repository.Root)
	if err != nil {
		commit = ""
	}
	record.Status.Observe(store.AnchorOK, commit, store.Now())
}

func (s *Service) captureGit(record *store.Annotation) []string {
	file := record.Spec.Target.File
	context, err := provenance.Capture(s.repository.Root, file, record.Status.LastSeenLine, record.Status.LastSeenLine)
	if err == nil {
		record.Spec.Git = context
		if provenance.WorktreeIsDirty(s.repository.Root, file) {
			return []string{fmt.Sprintf("%s has uncommitted changes, so commit %s does not describe what was annotated", file, context.Commit[:7])}
		}
		return nil
	}
	if errors.Is(err, provenance.ErrNoGit) {
		return []string{fmt.Sprintf("no git context recorded for %s", file)}
	}
	return []string{fmt.Sprintf("git context failed for %s: %v", file, err)}
}

func recordPath(id string) string {
	return path.Join(store.DirName, "annotations", id+".yaml")
}
