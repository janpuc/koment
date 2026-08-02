package store

import (
	"strings"
	"testing"
	"time"
)

const validCommit = "9f3c1a4d8e2b7c5a6f0d3e1b8c7a5f2d4e6b9c1a"

func TestGitContextRequiresAFullSHA(t *testing.T) {
	for _, commit := range []string{"9f3c1a4", "", "not-a-sha", strings.Repeat("z", 40)} {
		context := GitContext{Commit: commit, Path: "a.go"}
		if err := context.Validate(); err == nil {
			t.Errorf("commit %q should be rejected: an abbreviated SHA stops being unique", commit)
		}
	}
	if err := (GitContext{Commit: validCommit, Path: "a.go"}).Validate(); err != nil {
		t.Errorf("a full SHA should be accepted: %v", err)
	}
}

func TestGitContextRejectsImpossibleRanges(t *testing.T) {
	cases := map[string]GitContext{
		"end before start":  {Commit: validCommit, Path: "a.go", Line: 10, EndLine: 4},
		"end without start": {Commit: validCommit, Path: "a.go", EndLine: 4},
		"negative line":     {Commit: validCommit, Path: "a.go", Line: -1},
		"escaping path":     {Commit: validCommit, Path: "../outside.go", Line: 1},
	}
	for name, context := range cases {
		t.Run(name, func(t *testing.T) {
			if err := context.Validate(); err == nil {
				t.Error("want an error, got nil")
			}
		})
	}
}

func TestAuthorRequiresNameKindAndSource(t *testing.T) {
	valid := Author{Name: "Someone", Kind: AuthorHuman, Source: FromGitConfig}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid author rejected: %v", err)
	}

	cases := map[string]func(*Author){
		"no name":        func(a *Author) { a.Name = "  " },
		"unknown kind":   func(a *Author) { a.Kind = "robot" },
		"unknown source": func(a *Author) { a.Source = "vibes" },
		"missing kind":   func(a *Author) { a.Kind = "" },
		"missing source": func(a *Author) { a.Source = "" },
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			author := valid
			corrupt(&author)
			if err := author.Validate(); err == nil {
				t.Error("want an error, got nil")
			}
		})
	}
}

func TestUnverifiedIdentityIsNotProven(t *testing.T) {
	claimed := Author{Name: "Someone", Kind: AuthorHuman, Source: FromGitConfig}
	if claimed.IsProven() {
		t.Error("a git-config identity is a claim, not a proof")
	}
	if !(Author{Name: "S", Kind: AuthorHuman, Source: FromSession, Verified: "oidc"}).IsProven() {
		t.Error("a verified identity should report as proven")
	}
}

func TestProvenanceRoundTrips(t *testing.T) {
	s := newTestStore(t)
	want := &Record{
		Version: RecordVersion,
		File:    "a.go",
		Annotations: []Annotation{{
			ID:      "01JQ8ZK3M4N5P6R7S8T9V0W1X2",
			Scope:   ScopeFile,
			Kind:    KindWhy,
			Body:    "Entry point only.",
			Created: Date{time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)},
			Git:     &GitContext{Commit: validCommit, Path: "a.go", Line: 12, EndLine: 18},
			Author: &Author{
				Name: "Jan Pucilowski", Email: "janpuc@proton.me",
				Kind: AuthorHuman, Source: FromGitConfig,
			},
		}},
	}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}

	got, err := s.Load("a.go")
	if err != nil {
		t.Fatal(err)
	}
	recorded := got.Annotations[0]
	if recorded.Git == nil || recorded.Git.Commit != validCommit || recorded.Git.EndLine != 18 {
		t.Errorf("git context did not round trip: %+v", recorded.Git)
	}
	if recorded.Author == nil || recorded.Author.Email != "janpuc@proton.me" {
		t.Errorf("author did not round trip: %+v", recorded.Author)
	}
}

func TestProvenanceIsOptional(t *testing.T) {
	s := newTestStore(t)
	record := &Record{
		Version: RecordVersion,
		File:    "a.go",
		Annotations: []Annotation{{
			ID:      "01JQ8ZK3M4N5P6R7S8T9V0W1X2",
			Scope:   ScopeFile,
			Kind:    KindWhy,
			Body:    "Written before koment recorded provenance.",
			Created: Date{time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)},
		}},
	}
	if err := s.Save(record); err != nil {
		t.Fatalf("an annotation without provenance must still be valid: %v", err)
	}

	path, err := s.RecordPath("a.go")
	if err != nil {
		t.Fatal(err)
	}
	content := readFile(t, path)
	if strings.Contains(content, "git:") || strings.Contains(content, "author:") {
		t.Errorf("absent provenance must not be written as empty keys:\n%s", content)
	}
}
