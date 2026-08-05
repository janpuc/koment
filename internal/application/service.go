package application

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/provenance"
	"github.com/janpuc/koment/internal/repository"
	"github.com/janpuc/koment/internal/store"
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
	Kind    store.Kind
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
	if _, err := store.ParseKind(string(input.Kind)); err != nil {
		return Mutation{}, err
	}
	id, err := store.NewID(time.Now())
	if err != nil {
		return Mutation{}, err
	}
	record := store.Annotation{
		Version: store.RecordVersion, ID: id, File: file, Kind: input.Kind,
		Title: strings.TrimSpace(input.Title),
		Body:  store.WrapProse(input.Body), Created: store.Today(), Author: input.Author,
		Policy: input.Policy,
	}
	if err := s.anchor(&record, file, input.Excerpt); err != nil {
		return Mutation{}, err
	}
	warnings := s.captureGit(&record)
	if err := s.store.Save(&record); err != nil {
		return Mutation{}, err
	}
	return Mutation{Record: record, Path: recordPath(record.ID), Warnings: warnings}, nil
}

// Reanchor changes only an annotation's target.
func (s *Service) Reanchor(input ReanchorInput) (Mutation, error) {
	record, err := s.store.FindByID(input.ID)
	if err != nil {
		return Mutation{}, err
	}
	file := record.File
	if input.File != "" {
		if file, err = s.store.FromRoot(input.File); err != nil {
			return Mutation{}, err
		}
	}
	excerpt := input.Excerpt
	if excerpt == "" && record.Anchor.Scope == store.ScopeExcerpt {
		excerpt = record.Anchor.Excerpt
	}
	moved := *record
	if err := s.anchor(&moved, file, excerpt); err != nil {
		return Mutation{}, err
	}
	moved.File = file
	if err := s.store.Save(&moved); err != nil {
		return Mutation{}, err
	}
	return Mutation{Record: moved, Path: recordPath(moved.ID)}, nil
}

func (s *Service) anchor(record *store.Annotation, file, excerpt string) error {
	content, err := s.store.ReadSource(file)
	if err != nil {
		return fmt.Errorf("reading %s: %w", file, err)
	}
	if excerpt == "" {
		record.Anchor = store.Anchor{Scope: store.ScopeFile}
		return nil
	}
	captured, err := anchor.Capture(content, excerpt)
	if err != nil {
		lines := anchor.ExcerptLines(content, excerpt)
		switch len(lines) {
		case 0:
			return fmt.Errorf("excerpt not found in %s; it must match the file verbatim", file)
		default:
			return fmt.Errorf("excerpt matches %d places in %s (lines %v); extend it until it is unique", len(lines), file, lines)
		}
	}
	record.Anchor = captured
	return nil
}

func (s *Service) captureGit(record *store.Annotation) []string {
	context, err := provenance.Capture(s.repository.Root, record.File, record.Anchor.LastSeenLine, record.Anchor.LastSeenLine)
	if err == nil {
		record.Git = context
		if provenance.WorktreeIsDirty(s.repository.Root, record.File) {
			return []string{fmt.Sprintf("%s has uncommitted changes, so commit %s does not describe what was annotated", record.File, context.Commit[:7])}
		}
		return nil
	}
	if errors.Is(err, provenance.ErrNoGit) {
		return []string{fmt.Sprintf("no git context recorded for %s", record.File)}
	}
	return []string{fmt.Sprintf("git context failed for %s: %v", record.File, err)}
}

func recordPath(id string) string {
	return path.Join(store.DirName, "annotations", id+".yaml")
}
