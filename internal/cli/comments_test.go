package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koment-dev/koment/internal/policy"
)

func installPolicy(t *testing.T, root string) {
	t.Helper()
	if err := policy.Save(root, policy.Default()); err != nil {
		t.Fatal(err)
	}
}

func writeCommentedSource(t *testing.T, root string) {
	t.Helper()
	content := "package main\n\nfunc main() {\n\t// Start the service after configuration is loaded.\n\tserve()\n}\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCommentsCheckRejectsExplanatoryComment(t *testing.T) {
	root := repository(t)
	installPolicy(t, root)
	writeCommentedSource(t, root)

	got := koment(t, "comments", "check")
	if got.code != ExitFailure {
		t.Fatalf("want exit %d, got %d: %s", ExitFailure, got.code, got.output())
	}
	for _, wanted := range []string{"main.go:4", "comments convert", "explicitly acknowledged"} {
		if !strings.Contains(got.output(), wanted) {
			t.Errorf("output missing %q:\n%s", wanted, got.output())
		}
	}
}

func TestCommentsConvertRecordsBeforeRemovingComment(t *testing.T) {
	root := repository(t)
	installPolicy(t, root)
	writeCommentedSource(t, root)
	comment := "// Start the service after configuration is loaded."

	converted := koment(t, "comments", "convert", "main.go", "--excerpt", comment)
	if converted.code != ExitOK {
		t.Fatalf("convert exited %d: %s", converted.code, converted.output())
	}
	content, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), comment) {
		t.Fatalf("comment remains in source:\n%s", content)
	}
	shown := koment(t, "show", "main.go")
	if shown.code != ExitOK || !strings.Contains(shown.stdout, "Start the service after configuration is loaded.") {
		t.Fatalf("converted annotation unavailable: %s", shown.output())
	}
	checked := koment(t, "comments", "check")
	if checked.code != ExitOK {
		t.Fatalf("comment policy still fails: %s", checked.output())
	}
}

func TestCommentsAcknowledgeRequiresExplicitFlag(t *testing.T) {
	root := repository(t)
	installPolicy(t, root)
	writeCommentedSource(t, root)
	comment := "// Start the service after configuration is loaded."

	refused := koment(t, "comments", "acknowledge", "main.go", "--excerpt", comment, "--body", "An external generator requires the marker.")
	if refused.code != ExitUsage || !strings.Contains(refused.stderr, "--acknowledge-inline-comment") {
		t.Fatalf("missing acknowledgement was not refused: %s", refused.output())
	}
	accepted := koment(t, "comments", "acknowledge", "main.go", "--excerpt", comment,
		"--body", "An external generator requires this exact marker.", "--acknowledge-inline-comment")
	if accepted.code != ExitOK {
		t.Fatalf("acknowledge exited %d: %s", accepted.code, accepted.output())
	}
	checked := koment(t, "comments", "check")
	if checked.code != ExitOK {
		t.Fatalf("acknowledged comment still rejected: %s", checked.output())
	}
}
