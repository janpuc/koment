// Package store reads and writes the annotation records that live in .koment.
package store

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	yaml "go.yaml.in/yaml/v3"
)

const RecordVersion = 1

// TitleLimit keeps a title short enough to render beside code without being
// shortened, which is the only reason it exists (ADR 0115).
const TitleLimit = 72

const SchemaURL = "https://raw.githubusercontent.com/janpuc/koment/main/schema/annotation.schema.json"

type Kind string

const (
	KindWhy         Kind = "why"
	KindGotcha      Kind = "gotcha"
	KindInvariant   Kind = "invariant"
	KindAntiPattern Kind = "anti-pattern"
)

var Kinds = []Kind{KindWhy, KindGotcha, KindInvariant, KindAntiPattern}

func ParseKind(text string) (Kind, error) {
	for _, kind := range Kinds {
		if Kind(text) == kind {
			return kind, nil
		}
	}
	return "", fmt.Errorf("unknown kind %q, want one of %s", text, joinKinds())
}

func joinKinds() string {
	names := make([]string, len(Kinds))
	for index, kind := range Kinds {
		names[index] = string(kind)
	}
	return strings.Join(names, ", ")
}

type Scope string

const (
	ScopeFile    Scope = "file"
	ScopeExcerpt Scope = "excerpt"
)

func ParseScope(text string) (Scope, error) {
	switch Scope(text) {
	case ScopeFile:
		return ScopeFile, nil
	case ScopeExcerpt:
		return ScopeExcerpt, nil
	}
	return "", fmt.Errorf("unknown scope %q, want one of %s, %s", text, ScopeFile, ScopeExcerpt)
}

type Anchor struct {
	Scope        Scope  `yaml:"scope"`
	Excerpt      string `yaml:"excerpt,omitempty"`
	Before       string `yaml:"before,omitempty"`
	After        string `yaml:"after,omitempty"`
	LastSeenLine int    `yaml:"last_seen_line,omitempty"`
}

func (a Anchor) MarshalYAML() (any, error) {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	appendScalar := func(key, value, tag string, style yaml.Style) {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value, Style: style},
		)
	}
	appendScalar("scope", string(a.Scope), "!!str", 0)
	if a.Excerpt != "" {
		appendScalar("excerpt", a.Excerpt, "!!str", safeStringStyle(a.Excerpt))
	}
	if a.Before != "" {
		appendScalar("before", a.Before, "!!str", safeStringStyle(a.Before))
	}
	if a.After != "" {
		appendScalar("after", a.After, "!!str", safeStringStyle(a.After))
	}
	if a.LastSeenLine != 0 {
		appendScalar("last_seen_line", strconv.Itoa(a.LastSeenLine), "!!int", 0)
	}
	return node, nil
}

func safeStringStyle(value string) yaml.Style {
	if strings.Contains(value, "\t") {
		return yaml.DoubleQuotedStyle
	}
	if strings.Contains(value, "\n") {
		return yaml.LiteralStyle
	}
	return 0
}

func (a Anchor) Validate(id string) error {
	switch a.Scope {
	case ScopeFile:
		if a.Excerpt != "" || a.Before != "" || a.After != "" || a.LastSeenLine != 0 {
			return fmt.Errorf("annotation %s: file anchor must not carry excerpt context or a line", id)
		}
		return nil
	case ScopeExcerpt:
		if a.Excerpt == "" {
			return fmt.Errorf("annotation %s: excerpt anchor requires a non-empty excerpt", id)
		}
		if a.LastSeenLine < 1 {
			return fmt.Errorf("annotation %s: last_seen_line %d is not a positive line number", id, a.LastSeenLine)
		}
		if err := validateContext("before", a.Before); err != nil {
			return fmt.Errorf("annotation %s: %w", id, err)
		}
		if err := validateContext("after", a.After); err != nil {
			return fmt.Errorf("annotation %s: %w", id, err)
		}
		return nil
	default:
		_, err := ParseScope(string(a.Scope))
		return fmt.Errorf("annotation %s: %w", id, err)
	}
}

func validateContext(name, context string) error {
	if context == "" {
		return nil
	}
	if strings.Count(strings.TrimSuffix(context, "\n"), "\n") >= 3 {
		return fmt.Errorf("anchor.%s contains more than three lines", name)
	}
	return nil
}

type Policy struct {
	Exception    string `yaml:"exception"`
	Acknowledged bool   `yaml:"acknowledged"`
}

func (p Policy) Validate(annotation Annotation) error {
	if p.Exception != "inline-comment" || !p.Acknowledged {
		return fmt.Errorf("annotation %s: policy must explicitly acknowledge an inline-comment exception", annotation.ID)
	}
	if annotation.Kind != KindWhy || annotation.Anchor.Scope != ScopeExcerpt {
		return fmt.Errorf("annotation %s: inline-comment policy requires a why annotation with an excerpt anchor", annotation.ID)
	}
	return nil
}

type Annotation struct {
	Version int         `yaml:"version"`
	ID      string      `yaml:"id"`
	File    string      `yaml:"file"`
	Kind    Kind        `yaml:"kind"`
	Title   string      `yaml:"title,omitempty"`
	Body    string      `yaml:"body"`
	Created Date        `yaml:"created"`
	Anchor  Anchor      `yaml:"anchor"`
	Git     *GitContext `yaml:"git,omitempty"`
	Author  Author      `yaml:"author"`
	Policy  *Policy     `yaml:"policy,omitempty"`
}

// Headline is what a reader sees beside the code. A record written before
// titles existed still has to show something, so the first sentence of the body
// stands in, shortened at a word boundary. It is never written back: a derived
// title in the record would become a second copy of the body that drifts.
func (a Annotation) Headline() string {
	if title := strings.TrimSpace(a.Title); title != "" {
		return title
	}
	return shorten(firstSentence(a.Body), TitleLimit)
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
		return strings.TrimRight(string(runes)[:space], " ,;:") + "\u2026"
	}
	return strings.TrimRight(string(runes), " ,;:") + "\u2026"
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
	if a.Version != RecordVersion {
		return fmt.Errorf("annotation %s has version %d, want %d", a.ID, a.Version, RecordVersion)
	}
	if !ValidID(a.ID) {
		return fmt.Errorf("annotation id %q is not a canonical ULID", a.ID)
	}
	if _, err := validSourcePath(a.File); err != nil {
		return fmt.Errorf("annotation %s file: %w", a.ID, err)
	}
	if _, err := ParseKind(string(a.Kind)); err != nil {
		return fmt.Errorf("annotation %s: %w", a.ID, err)
	}
	if err := validTitle(a.ID, a.Title); err != nil {
		return err
	}
	if strings.TrimSpace(a.Body) == "" {
		return fmt.Errorf("annotation %s: empty body", a.ID)
	}
	if a.Created.IsZero() {
		return fmt.Errorf("annotation %s: missing created date", a.ID)
	}
	if err := a.Anchor.Validate(a.ID); err != nil {
		return err
	}
	if a.Git != nil {
		if err := a.Git.Validate(); err != nil {
			return fmt.Errorf("annotation %s: %w", a.ID, err)
		}
	}
	if err := a.Author.Validate(); err != nil {
		return fmt.Errorf("annotation %s: %w", a.ID, err)
	}
	if a.Policy != nil {
		if err := a.Policy.Validate(a); err != nil {
			return err
		}
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
