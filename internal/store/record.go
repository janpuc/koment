// Package store reads and writes the annotation records that live in .koment.
package store

import (
	"fmt"
	"strings"

	"github.com/janpuc/koment/internal/api"
)

// APIVersion is matched exactly. A record carrying anything else is refused
// rather than guessed at.
const APIVersion = api.Version

// KindAnnotation is the resource kind of an annotation record.
const KindAnnotation = "Annotation"

// TitleLimit keeps a title short enough to render beside code without being
// shortened, which is the only reason it exists (ADR 0115).
const TitleLimit = 72

const SchemaURL = api.SchemaBase + "annotation.schema.json"

// Type is the category of rationale a record carries.
type Type string

const (
	TypeWhy         Type = "why"
	TypeGotcha      Type = "gotcha"
	TypeInvariant   Type = "invariant"
	TypeAntiPattern Type = "anti-pattern"
)

var Types = []Type{TypeWhy, TypeGotcha, TypeInvariant, TypeAntiPattern}

func ParseType(text string) (Type, error) {
	for _, candidate := range Types {
		if Type(text) == candidate {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unknown type %q, want one of %s", text, joinTypes())
}

func joinTypes() string {
	names := make([]string, len(Types))
	for index, annotationType := range Types {
		names[index] = string(annotationType)
	}
	return strings.Join(names, ", ")
}

// AnchorStatus is the verdict of resolving an anchor against a file. It is
// computed by reading the file, never by trusting a stored value.
type AnchorStatus string

const (
	AnchorOK        AnchorStatus = "ok"
	AnchorAmbiguous AnchorStatus = "ambiguous"
	AnchorDrifted   AnchorStatus = "drifted"
	AnchorOrphaned  AnchorStatus = "orphaned"
)

var AnchorStatuses = []AnchorStatus{AnchorOK, AnchorAmbiguous, AnchorDrifted, AnchorOrphaned}

func (s AnchorStatus) IsFailure() bool {
	return s == AnchorAmbiguous || s == AnchorDrifted || s == AnchorOrphaned
}

func ParseAnchorStatus(text string) (AnchorStatus, error) {
	for _, candidate := range AnchorStatuses {
		if AnchorStatus(text) == candidate {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unknown resolution %q", text)
}

// Annotation is one rationale record on disk.
type Annotation struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
	Status     Status   `yaml:"status,omitempty"`
}

// Metadata identifies the record. Kubernetes names this field name; a ULID is
// not a DNS-1123 name, so koment diverges deliberately and calls it id. Do not
// align it without a migration.
type Metadata struct {
	ID      string    `yaml:"id"`
	Created Timestamp `yaml:"created"`
}

// Target is what the annotation is about. It is a mapping rather than a bare
// path so that a function or a member can join the file without reshaping the
// record again.
type Target struct {
	File string `yaml:"file"`
}

// Spec is the authored intent: everything a person or agent decided.
type Spec struct {
	Target Target      `yaml:"target"`
	Type   Type        `yaml:"type"`
	Title  string      `yaml:"title,omitempty"`
	Body   string      `yaml:"body"`
	Anchor Anchor      `yaml:"anchor"`
	Author Author      `yaml:"author"`
	Git    *GitContext `yaml:"git,omitempty"`
	Policy *Policy     `yaml:"policy,omitempty"`
}

// Status is what the last write observed, stamped with the commit it observed
// it at. Nothing reads it back as a verdict: a reader resolves the anchor
// against the file in front of it, and ResolvedCommit is what lets that reader
// see how old the recorded observation is.
type Status struct {
	LastSeenLine   int          `yaml:"lastSeenLine,omitempty"`
	Resolution     AnchorStatus `yaml:"resolution,omitempty"`
	ResolvedAt     Timestamp    `yaml:"resolvedAt,omitempty"`
	ResolvedCommit string       `yaml:"resolvedCommit,omitempty"`
}

// Observe records a resolution, leaving ResolvedAt alone when the verdict and
// the commit are already the ones recorded. ResolvedAt then answers "since
// when has this been true" instead of "when did a command last run".
func (s *Status) Observe(resolution AnchorStatus, commit string, at Timestamp) {
	if s.Resolution == resolution && s.ResolvedCommit == commit {
		return
	}
	s.Resolution = resolution
	s.ResolvedCommit = commit
	s.ResolvedAt = at
}

func (s Status) Validate(id string, scope Scope) error {
	switch scope {
	case ScopeFile:
		if s.LastSeenLine != 0 {
			return fmt.Errorf("annotation %s: a file-scoped record has no line to observe", id)
		}
	case ScopeExcerpt:
		if s.LastSeenLine < 1 {
			return fmt.Errorf("annotation %s: status.lastSeenLine %d is not a positive line number", id, s.LastSeenLine)
		}
	}
	if s.Resolution != "" {
		if _, err := ParseAnchorStatus(string(s.Resolution)); err != nil {
			return fmt.Errorf("annotation %s: %w", id, err)
		}
	}
	if (s.Resolution == "") != s.ResolvedAt.IsZero() {
		return fmt.Errorf("annotation %s: status.resolution and status.resolvedAt are recorded together or not at all", id)
	}
	if s.ResolvedCommit != "" && !fullCommitSHA.MatchString(s.ResolvedCommit) {
		return fmt.Errorf("annotation %s: status.resolvedCommit %q is not a full SHA", id, s.ResolvedCommit)
	}
	return nil
}

type Policy struct {
	Exception    string `yaml:"exception"`
	Acknowledged bool   `yaml:"acknowledged"`
}

func (p Policy) Validate(annotation Annotation) error {
	if p.Exception != "inline-comment" || !p.Acknowledged {
		return fmt.Errorf("annotation %s: policy must explicitly acknowledge an inline-comment exception", annotation.Metadata.ID)
	}
	if annotation.Spec.Type != TypeWhy || annotation.Spec.Anchor.Scope != ScopeExcerpt {
		return fmt.Errorf("annotation %s: inline-comment policy requires a why annotation with an excerpt anchor", annotation.Metadata.ID)
	}
	return nil
}

// Headline is what a reader sees beside the code. A record written before
// titles existed still has to show something, so the first sentence of the body
// stands in, shortened at a word boundary. It is never written back: a derived
// title in the record would become a second copy of the body that drifts.
func (a Annotation) Headline() string {
	if title := strings.TrimSpace(a.Spec.Title); title != "" {
		return title
	}
	return shorten(firstSentence(a.Spec.Body), TitleLimit)
}

func firstSentence(body string) string {
	flattened := strings.Join(strings.Fields(body), " ")
	for index, character := range flattened {
		if character != '.' && character != '!' && character != '?' {
			continue
		}
		if index+1 >= len(flattened) || flattened[index+1] == ' ' {
			return flattened[:index]
		}
	}
	return flattened
}

func shorten(text string, limit int) string {
	if len([]rune(text)) <= limit {
		return text
	}
	runes := []rune(text)[:limit]
	if space := strings.LastIndex(string(runes), " "); space > limit/2 {
		return strings.TrimRight(string(runes)[:space], " ,;:") + "…"
	}
	return strings.TrimRight(string(runes), " ,;:") + "…"
}

func validTitle(id, title string) error {
	if title == "" {
		return nil
	}
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("annotation %s: blank title", id)
	}
	if strings.ContainsAny(title, "\n\r") {
		return fmt.Errorf("annotation %s: a title is one line", id)
	}
	if count := len([]rune(title)); count > TitleLimit {
		return fmt.Errorf("annotation %s: title is %d characters, the limit is %d so it never needs shortening", id, count, TitleLimit)
	}
	return nil
}

func (a Annotation) Validate() error {
	if a.APIVersion != APIVersion {
		return fmt.Errorf("annotation %s has apiVersion %q, want %q", a.Metadata.ID, a.APIVersion, APIVersion)
	}
	if a.Kind != KindAnnotation {
		return fmt.Errorf("annotation %s has kind %q, want %q", a.Metadata.ID, a.Kind, KindAnnotation)
	}
	if err := a.Metadata.Validate(); err != nil {
		return err
	}
	if err := a.Spec.Validate(a.Metadata.ID); err != nil {
		return err
	}
	if a.Spec.Policy != nil {
		if err := a.Spec.Policy.Validate(a); err != nil {
			return err
		}
	}
	return a.Status.Validate(a.Metadata.ID, a.Spec.Anchor.Scope)
}

func (m Metadata) Validate() error {
	if !ValidID(m.ID) {
		return fmt.Errorf("annotation id %q is not a canonical ULID", m.ID)
	}
	if m.Created.IsZero() {
		return fmt.Errorf("annotation %s: missing metadata.created", m.ID)
	}
	return nil
}

func (s Spec) Validate(id string) error {
	if _, err := validSourcePath(s.Target.File); err != nil {
		return fmt.Errorf("annotation %s target.file: %w", id, err)
	}
	if _, err := ParseType(string(s.Type)); err != nil {
		return fmt.Errorf("annotation %s: %w", id, err)
	}
	if err := validTitle(id, s.Title); err != nil {
		return err
	}
	if strings.TrimSpace(s.Body) == "" {
		return fmt.Errorf("annotation %s: empty body", id)
	}
	if err := s.Anchor.Validate(id); err != nil {
		return err
	}
	if s.Git != nil {
		if err := s.Git.Validate(); err != nil {
			return fmt.Errorf("annotation %s: %w", id, err)
		}
	}
	if err := s.Author.Validate(); err != nil {
		return fmt.Errorf("annotation %s: %w", id, err)
	}
	return nil
}
