// Package store reads and writes the annotation records that live in .koment.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const RecordVersion = 1

type Kind string

const (
	KindWhy         Kind = "why"
	KindGotcha      Kind = "gotcha"
	KindInvariant   Kind = "invariant"
	KindAntiPattern Kind = "anti-pattern"
)

var Kinds = []Kind{KindWhy, KindGotcha, KindInvariant, KindAntiPattern}

func ParseKind(s string) (Kind, error) {
	for _, k := range Kinds {
		if Kind(s) == k {
			return k, nil
		}
	}
	return "", fmt.Errorf("unknown kind %q, want one of %s", s, joinKinds())
}

func joinKinds() string {
	names := make([]string, len(Kinds))
	for i, k := range Kinds {
		names[i] = string(k)
	}
	return strings.Join(names, ", ")
}

type Scope string

const (
	ScopeFile    Scope = "file"
	ScopeExcerpt Scope = "excerpt"
)

func ParseScope(s string) (Scope, error) {
	switch Scope(s) {
	case ScopeFile:
		return ScopeFile, nil
	case ScopeExcerpt:
		return ScopeExcerpt, nil
	}
	return "", fmt.Errorf("unknown scope %q, want one of %s, %s", s, ScopeFile, ScopeExcerpt)
}

type Annotation struct {
	ID            string `yaml:"id"`
	Scope         Scope  `yaml:"scope"`
	Excerpt       string `yaml:"excerpt,omitempty"`
	ExcerptSHA256 string `yaml:"excerpt_sha256,omitempty"`
	LastSeenLine  int    `yaml:"last_seen_line,omitempty"`
	Kind          Kind   `yaml:"kind"`
	Body          string `yaml:"body"`
	Created       Date   `yaml:"created"`
}

func ExcerptSHA256(excerpt string) string {
	sum := sha256.Sum256([]byte(excerpt))
	return hex.EncodeToString(sum[:])
}

func (a Annotation) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("annotation has no id")
	}
	if _, err := ParseKind(string(a.Kind)); err != nil {
		return fmt.Errorf("annotation %s: %w", a.ID, err)
	}
	if strings.TrimSpace(a.Body) == "" {
		return fmt.Errorf("annotation %s: empty body", a.ID)
	}
	if a.Created.IsZero() {
		return fmt.Errorf("annotation %s: missing created date", a.ID)
	}
	return a.validateAnchor()
}

func (a Annotation) validateAnchor() error {
	switch a.Scope {
	case ScopeFile:
		if a.Excerpt != "" || a.ExcerptSHA256 != "" {
			return fmt.Errorf("annotation %s: scope %s must not carry an excerpt", a.ID, ScopeFile)
		}
		if a.LastSeenLine != 0 {
			return fmt.Errorf("annotation %s: scope %s has nothing to locate, so last_seen_line is meaningless", a.ID, ScopeFile)
		}
		return nil
	case ScopeExcerpt:
		if a.Excerpt == "" {
			return fmt.Errorf("annotation %s: scope %s requires a non-empty excerpt", a.ID, ScopeExcerpt)
		}
		if a.LastSeenLine < 0 {
			return fmt.Errorf("annotation %s: last_seen_line %d is not a line number", a.ID, a.LastSeenLine)
		}
		if want := ExcerptSHA256(a.Excerpt); a.ExcerptSHA256 != want {
			return fmt.Errorf("annotation %s: excerpt_sha256 is %s but the stored excerpt hashes to %s",
				a.ID, a.ExcerptSHA256, want)
		}
		return nil
	}
	_, err := ParseScope(string(a.Scope))
	return fmt.Errorf("annotation %s: %w", a.ID, err)
}

type Record struct {
	Version     int          `yaml:"version"`
	File        string       `yaml:"file"`
	Annotations []Annotation `yaml:"annotations"`
}

func (r Record) Validate() error {
	if r.Version != RecordVersion {
		return fmt.Errorf("record for %s has version %d, want %d", r.File, r.Version, RecordVersion)
	}
	if r.File == "" {
		return fmt.Errorf("record has no file path")
	}
	if len(r.Annotations) == 0 {
		return fmt.Errorf("record for %s has no annotations", r.File)
	}
	seen := make(map[string]struct{}, len(r.Annotations))
	for _, a := range r.Annotations {
		if err := a.Validate(); err != nil {
			return fmt.Errorf("record for %s: %w", r.File, err)
		}
		if _, duplicate := seen[a.ID]; duplicate {
			return fmt.Errorf("record for %s: duplicate annotation id %s", r.File, a.ID)
		}
		seen[a.ID] = struct{}{}
	}
	return nil
}

// Date is a calendar date with no time or zone, written as YYYY-MM-DD.
type Date struct{ time.Time }

const dateLayout = "2006-01-02"

func Today() Date { return Date{time.Now().UTC().Truncate(24 * time.Hour)} }

func (d Date) MarshalYAML() (any, error) { return d.Format(dateLayout), nil }

func (d *Date) UnmarshalYAML(unmarshal func(any) error) error {
	var text string
	if err := unmarshal(&text); err != nil {
		return fmt.Errorf("created must be a %s date: %w", dateLayout, err)
	}
	parsed, err := time.Parse(dateLayout, text)
	if err != nil {
		return fmt.Errorf("created %q is not a %s date", text, dateLayout)
	}
	d.Time = parsed
	return nil
}
