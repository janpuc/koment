package provenance

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/janpuc/koment/internal/store"
)

func repository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "--initial-branch=main"},
		{"config", "user.name", "Fixture Author"},
		{"config", "user.email", "fixture@example.test"},
		{"config", "commit.gpgsign", "false"},
	} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	return root
}

func commitFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", name}, {"commit", "-m", "add " + name}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
}

func TestCaptureRecordsTheCommitAndPassesValidation(t *testing.T) {
	root := repository(t)
	commitFile(t, root, "main.go", "package main\n\nfunc main() {}\n")

	context, err := Capture(root, "main.go", 3, 3)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if err := context.Validate(); err != nil {
		t.Errorf("captured context does not validate: %v", err)
	}
	if len(context.Commit) != 40 && len(context.Commit) != 64 {
		t.Errorf("commit %q is not a full SHA", context.Commit)
	}
	if context.Path != "main.go" || context.Line != 3 {
		t.Errorf("unexpected context: %+v", context)
	}
	if context.EndLine != 0 {
		t.Errorf("a single line should not repeat itself in end_line, got %d", context.EndLine)
	}
}

func TestCaptureRefusesRatherThanGuessing(t *testing.T) {
	t.Run("not a repository", func(t *testing.T) {
		if _, err := Capture(t.TempDir(), "main.go", 1, 1); !errors.Is(err, ErrNoGit) {
			t.Errorf("want ErrNoGit, got %v", err)
		}
	})

	t.Run("file is untracked", func(t *testing.T) {
		root := repository(t)
		commitFile(t, root, "tracked.go", "package main\n")
		if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Capture(root, "new.go", 1, 1); !errors.Is(err, ErrNoGit) {
			t.Errorf("an untracked file has no commit to reference; want ErrNoGit, got %v", err)
		}
	})
}

func TestWorktreeIsDirtyDetectsUncommittedChanges(t *testing.T) {
	root := repository(t)
	commitFile(t, root, "main.go", "package main\n")

	if WorktreeIsDirty(root, "main.go") {
		t.Error("a freshly committed file is not dirty")
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main // edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !WorktreeIsDirty(root, "main.go") {
		t.Error("an edited file is dirty, and the captured commit no longer describes it")
	}
}

func TestIdentityFromGitIsAClaimNotAProof(t *testing.T) {
	root := repository(t)

	author, err := IdentityFromGit(root)
	if err != nil {
		t.Fatalf("IdentityFromGit: %v", err)
	}
	if author.Name != "Fixture Author" || author.Email != "fixture@example.test" {
		t.Errorf("unexpected identity: %+v", author)
	}
	if author.Source != store.FromGitConfig {
		t.Errorf("want source %s, got %s", store.FromGitConfig, author.Source)
	}
	if author.IsProven() {
		t.Error("a git-config identity must not report as proven")
	}
	if err := author.Validate(); err != nil {
		t.Errorf("captured identity does not validate: %v", err)
	}
}

func TestParseAuthorAcceptsNameAndOptionalEmail(t *testing.T) {
	withEmail, err := ParseAuthor("Ada Lovelace <ada@example.test>", store.AuthorHuman)
	if err != nil {
		t.Fatal(err)
	}
	if withEmail.Name != "Ada Lovelace" || withEmail.Email != "ada@example.test" {
		t.Errorf("unexpected: %+v", withEmail)
	}
	if withEmail.Source != store.FromExplicit {
		t.Errorf("want source %s, got %s", store.FromExplicit, withEmail.Source)
	}

	nameOnly, err := ParseAuthor("Ada Lovelace", store.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if nameOnly.Email != "" || nameOnly.Kind != store.AuthorAgent {
		t.Errorf("unexpected: %+v", nameOnly)
	}

	if _, err := ParseAuthor("  ", store.AuthorHuman); err == nil {
		t.Error("an empty author should be refused")
	}
}
