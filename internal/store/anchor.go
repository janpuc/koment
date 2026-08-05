package store

import (
	"fmt"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

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

// Anchor is where an annotation aims. It carries no line, because a line is
// something a reader observes rather than something the author decided; that
// lives in Status.
type Anchor struct {
	Scope   Scope  `yaml:"scope"`
	Excerpt string `yaml:"excerpt,omitempty"`
	Before  string `yaml:"before,omitempty"`
	After   string `yaml:"after,omitempty"`
}

func (a Anchor) MarshalYAML() (any, error) {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	appendScalar := func(key, value string, style yaml.Style) {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, Style: style},
		)
	}
	appendScalar("scope", string(a.Scope), 0)
	if a.Excerpt != "" {
		appendScalar("excerpt", a.Excerpt, safeStringStyle(a.Excerpt))
	}
	if a.Before != "" {
		appendScalar("before", a.Before, safeStringStyle(a.Before))
	}
	if a.After != "" {
		appendScalar("after", a.After, safeStringStyle(a.After))
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
		if a.Excerpt != "" || a.Before != "" || a.After != "" {
			return fmt.Errorf("annotation %s: file anchor must not carry excerpt context", id)
		}
		return nil
	case ScopeExcerpt:
		if a.Excerpt == "" {
			return fmt.Errorf("annotation %s: excerpt anchor requires a non-empty excerpt", id)
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
