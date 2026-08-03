package application

import (
	"fmt"

	"github.com/janpuc/koment/internal/commentpolicy"
	"github.com/janpuc/koment/internal/policy"
	"github.com/janpuc/koment/internal/store"
)

// ConvertCommentInput records explanatory comment intent before removing it.
type ConvertCommentInput struct {
	File    string
	Comment string
	Kind    store.Kind
	Author  store.Author
}

// AcknowledgeCommentInput explicitly retains one exceptional inline comment.
type AcknowledgeCommentInput struct {
	File         string
	Comment      string
	Body         string
	Author       store.Author
	Acknowledged bool
}

// ConvertComment durably records rationale before editing the source.
func (s *Service) ConvertComment(input ConvertCommentInput) (Mutation, error) {
	file, err := s.store.FromRoot(input.File)
	if err != nil {
		return Mutation{}, err
	}
	content, err := s.store.ReadSource(file)
	if err != nil {
		return Mutation{}, fmt.Errorf("reading %s: %w", file, err)
	}
	comment, err := commentpolicy.Find(file, content, input.Comment)
	if err != nil {
		return Mutation{}, err
	}
	conversion, err := commentpolicy.Convert(content, comment)
	if err != nil {
		return Mutation{}, err
	}
	kind := input.Kind
	if kind == "" {
		kind = store.KindWhy
	}
	created, err := s.Add(AddInput{
		File: file, Excerpt: conversion.Excerpt, Kind: kind,
		Body: conversion.Body, Author: input.Author,
	})
	if err != nil {
		return Mutation{}, err
	}
	if err := s.store.WriteSource(file, conversion.Content); err != nil {
		return Mutation{}, fmt.Errorf("annotation %s was written before source conversion failed: %w", created.Record.ID, err)
	}
	reanchored, err := s.Reanchor(ReanchorInput{ID: created.Record.ID, File: file, Excerpt: conversion.Excerpt})
	if err != nil {
		return Mutation{}, fmt.Errorf("comment was converted and annotation %s exists, but refreshing its line failed: %w", created.Record.ID, err)
	}
	reanchored.Warnings = created.Warnings
	return reanchored, nil
}

// AcknowledgeComment creates the exact policy record required to retain a comment.
func (s *Service) AcknowledgeComment(input AcknowledgeCommentInput) (Mutation, error) {
	if !input.Acknowledged {
		return Mutation{}, fmt.Errorf("retaining an inline comment requires explicit acknowledgement")
	}
	file, err := s.store.FromRoot(input.File)
	if err != nil {
		return Mutation{}, err
	}
	content, err := s.store.ReadSource(file)
	if err != nil {
		return Mutation{}, fmt.Errorf("reading %s: %w", file, err)
	}
	comment, err := commentpolicy.Find(file, content, input.Comment)
	if err != nil {
		return Mutation{}, err
	}
	excerpt, err := commentpolicy.AcknowledgementExcerpt(content, comment)
	if err != nil {
		return Mutation{}, err
	}
	return s.Add(AddInput{
		File: file, Excerpt: excerpt, Kind: store.KindWhy, Body: input.Body, Author: input.Author,
		Policy: &store.Policy{Exception: "inline-comment", Acknowledged: true},
	})
}

// CheckComments applies the configured repository policy.
func (s *Service) CheckComments(requested []string) ([]commentpolicy.Violation, error) {
	configured, err := policy.Load(s.repository.Root)
	if err != nil {
		return nil, err
	}
	return commentpolicy.Check(s.repository.Root, configured, requested)
}
