// Package provenance captures where an annotation came from: the git state it
// was written against, and who wrote it.
package provenance

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/koment-dev/koment/internal/store"
)

// ErrNoGit means git could not answer, which is a normal outcome rather than a
// failure: koment works in a directory that is not a repository.
var ErrNoGit = errors.New("no usable git context")

// Capture records the commit an annotation was written against. It reports
// ErrNoGit rather than guessing when git cannot answer, because a fabricated
// commit reference is worse than an absent one.
func Capture(root, file string, line, endLine int) (*store.GitContext, error) {
	commit, err := Head(root)
	if err != nil {
		return nil, err
	}
	if tracked, err := git(root, "ls-files", "--error-unmatch", "--", file); err != nil || tracked == "" {
		return nil, ErrNoGit
	}

	return &store.GitContext{
		Commit:  commit,
		Path:    file,
		Line:    line,
		EndLine: endLineOrZero(line, endLine),
	}, nil
}

func endLineOrZero(line, endLine int) int {
	if endLine == line {
		return 0
	}
	return endLine
}

// Head is the full commit a record is stamped against. A record never stores
// the abbreviated form, because an abbreviation stops being unique as history
// grows.
func Head(root string) (string, error) {
	commit, err := git(root, "rev-parse", "HEAD")
	if err != nil || commit == "" {
		return "", ErrNoGit
	}
	return commit, nil
}

// HeadCommit is the abbreviated commit a snapshot was taken at. It reports
// ErrNoGit rather than an empty string, so a caller that needs to name the
// commit cannot mistake "not a repository" for "no commit".
func HeadCommit(root string) (string, error) {
	commit, err := git(root, "rev-parse", "--short", "HEAD")
	if err != nil || commit == "" {
		return "", ErrNoGit
	}
	return commit, nil
}

// WorktreeIsDirty reports whether the file has uncommitted changes, which makes
// the captured commit describe something other than what was annotated.
func WorktreeIsDirty(root, file string) bool {
	changed, err := git(root, "status", "--porcelain", "--", file)
	return err == nil && changed != ""
}

// TreeIsDirty is the same question asked of the whole repository, which is what
// a snapshot of every file has to ask before naming a commit.
func TreeIsDirty(root string) bool {
	changed, err := git(root, "status", "--porcelain")
	return err == nil && changed != ""
}

// IdentityFromGit reads the committer identity git would use. It is a claim,
// not a proof, and says so through its source.
func IdentityFromGit(root string) (*store.Author, error) {
	name, err := git(root, "config", "user.name")
	if err != nil || name == "" {
		return nil, errors.New("no git user.name configured; set one or pass --author")
	}
	email, emailErr := git(root, "config", "user.email")
	if emailErr != nil {
		email = ""
	}

	return &store.Author{
		Name:   name,
		Email:  email,
		Kind:   store.AuthorHuman,
		Source: store.FromGitConfig,
	}, nil
}

// ParseAuthor reads an explicit "Name <email>" identity.
func ParseAuthor(text string, kind store.AuthorKind) (*store.Author, error) {
	name, email, found := strings.Cut(strings.TrimSpace(text), "<")
	author := &store.Author{
		Name:   strings.TrimSpace(name),
		Kind:   kind,
		Source: store.FromExplicit,
	}
	if found {
		author.Email = strings.TrimSpace(strings.TrimSuffix(email, ">"))
	}
	if author.Name == "" {
		return nil, errors.New(`--author must look like "Name" or "Name <email>"`)
	}
	return author, nil
}

func git(root string, args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
