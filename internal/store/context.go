package store

import (
	"fmt"
	"regexp"
	"strings"
)

// GitContext is what was true when an annotation was written. It is written
// once and never rewritten: reanchor changes where an annotation aims, not
// what it witnessed. ADR 0014.
type GitContext struct {
	Commit  string `yaml:"commit"`
	Path    string `yaml:"path"`
	Line    int    `yaml:"line,omitempty"`
	EndLine int    `yaml:"endLine,omitempty"`
}

var fullCommitSHA = regexp.MustCompile(`^[0-9a-f]{40}$|^[0-9a-f]{64}$`)

func (g GitContext) Validate() error {
	if !fullCommitSHA.MatchString(g.Commit) {
		return fmt.Errorf("git.commit %q is not a full SHA; an abbreviated one stops being unique as history grows", g.Commit)
	}
	if _, err := validSourcePath(g.Path); err != nil {
		return fmt.Errorf("git.path: %w", err)
	}
	switch {
	case g.Line < 0 || g.EndLine < 0:
		return fmt.Errorf("git.line %d-%d is not a line range", g.Line, g.EndLine)
	case g.EndLine != 0 && g.Line == 0:
		return fmt.Errorf("git.end_line %d given without a start line", g.EndLine)
	case g.EndLine != 0 && g.EndLine < g.Line:
		return fmt.Errorf("git.end_line %d is before git.line %d", g.EndLine, g.Line)
	}
	return nil
}

// AuthorKind separates a person from an agent, because an agent weighing an
// annotation needs to know whether another agent wrote it.
type AuthorKind string

const (
	AuthorHuman   AuthorKind = "human"
	AuthorAgent   AuthorKind = "agent"
	AuthorUnknown AuthorKind = "unknown"
)

// IdentitySource records how much an identity is worth. Git config asserts an
// identity; it does not prove one.
type IdentitySource string

const (
	FromGitConfig   IdentitySource = "git-config"
	FromSession     IdentitySource = "session"
	FromExplicit    IdentitySource = "explicit"
	FromMigration   IdentitySource = "migration"
	FromOIDCProxy   IdentitySource = "oidc-proxy"
	FromScopedAgent IdentitySource = "bearer-credential"
)

// Author is a claim, not a proof, unless Verified says otherwise. ADR 0015.
type Author struct {
	Name     string         `yaml:"name"`
	Email    string         `yaml:"email,omitempty"`
	Kind     AuthorKind     `yaml:"kind"`
	Source   IdentitySource `yaml:"source"`
	Account  string         `yaml:"account,omitempty"`
	Verified string         `yaml:"verified,omitempty"`
}

func (a Author) Validate() error {
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("author has no name")
	}
	switch a.Kind {
	case AuthorHuman, AuthorAgent, AuthorUnknown:
	default:
		return fmt.Errorf("author kind %q, want one of %s, %s, %s", a.Kind, AuthorHuman, AuthorAgent, AuthorUnknown)
	}
	switch a.Source {
	case FromGitConfig, FromSession, FromExplicit, FromMigration, FromOIDCProxy, FromScopedAgent:
	default:
		return fmt.Errorf("author source %q is unknown", a.Source)
	}
	if a.Kind == AuthorUnknown && a.Source != FromMigration {
		return fmt.Errorf("unknown author must have migration as its source")
	}
	if a.Kind != AuthorUnknown && a.Source == FromMigration {
		return fmt.Errorf("migration source requires an unknown author")
	}
	return nil
}

// IsProven reports whether the identity was verified by some mechanism. An
// unverified identity must not be rendered as though it were checked.
func (a Author) IsProven() bool { return a.Verified != "" }
