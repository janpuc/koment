package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janpuc/koment/internal/store"
)

const source = "package main\n\nfunc main() {\n\tserve()\n}\n"

type result struct {
	code   int
	stdout string
	stderr string
}

func (r result) output() string { return r.stdout + r.stderr }

func repository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, store.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	return root
}

func koment(t *testing.T, args ...string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := Environment{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}

	unreachable := func(name string) Server {
		return func([]string, io.Writer) error { t.Fatalf("%s must not be reached", name); return nil }
	}
	code := Run(args, env, Servers{MCP: unreachable("mcp"), UI: unreachable("ui")})
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func addServeAnnotation(t *testing.T) result {
	t.Helper()
	return koment(t, "add", "main.go",
		"--excerpt", "\tserve()",
		"--kind", "invariant",
		"--body", "serve must be the last call: it blocks until the process is signalled.")
}

func TestAddThenShowThenCheckPasses(t *testing.T) {
	repository(t)

	if got := addServeAnnotation(t); got.code != ExitOK {
		t.Fatalf("add exited %d: %s", got.code, got.output())
	}

	shown := koment(t, "show", "main.go")
	if shown.code != ExitOK {
		t.Fatalf("show exited %d: %s", shown.code, shown.output())
	}
	for _, want := range []string{"main.go:4", "ok", "invariant", "blocks until the process is signalled"} {
		if !strings.Contains(shown.stdout, want) {
			t.Errorf("show output missing %q:\n%s", want, shown.stdout)
		}
	}

	checked := koment(t, "check")
	if checked.code != ExitOK {
		t.Fatalf("check exited %d on an intact anchor: %s", checked.code, checked.output())
	}
	if !strings.Contains(checked.stdout, "1 ok") {
		t.Errorf("check did not report the annotation as ok:\n%s", checked.stdout)
	}
}

func TestCheckExitsNonZeroOnDrift(t *testing.T) {
	root := repository(t)
	if got := addServeAnnotation(t); got.code != ExitOK {
		t.Fatalf("add exited %d: %s", got.code, got.output())
	}

	edited := strings.Replace(source, "\tserve()", "\tserveForever()", 1)
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	got := koment(t, "check")
	if got.code != ExitFailure {
		t.Fatalf("want exit %d on drift, got %d:\n%s", ExitFailure, got.code, got.output())
	}
	if !strings.Contains(got.stdout, "drifted") {
		t.Errorf("check did not name the drifted annotation:\n%s", got.stdout)
	}
	if !strings.Contains(got.stderr, "no longer resolve") {
		t.Errorf("check did not explain the failure on stderr:\n%s", got.stderr)
	}
}

func TestCheckExitsNonZeroWhenTheFileIsDeleted(t *testing.T) {
	root := repository(t)
	if got := addServeAnnotation(t); got.code != ExitOK {
		t.Fatalf("add exited %d: %s", got.code, got.output())
	}
	if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}

	got := koment(t, "check")
	if got.code != ExitFailure {
		t.Fatalf("want exit %d for an orphan, got %d:\n%s", ExitFailure, got.code, got.output())
	}
	if !strings.Contains(got.stdout, "orphaned") {
		t.Errorf("check did not name the orphaned annotation:\n%s", got.stdout)
	}
}

func TestCheckIsCleanOnARepositoryWithNoAnnotations(t *testing.T) {
	repository(t)

	got := koment(t, "check")
	if got.code != ExitOK {
		t.Fatalf("want exit %d, got %d:\n%s", ExitOK, got.code, got.output())
	}
	if !strings.Contains(got.stdout, "0 annotations") {
		t.Errorf("want a zero summary, got:\n%s", got.stdout)
	}
}

func TestCheckNarrowsToTheGivenPaths(t *testing.T) {
	root := repository(t)
	nested := filepath.Join(root, "internal")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "serve.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := addServeAnnotation(t); got.code != ExitOK {
		t.Fatalf("add exited %d: %s", got.code, got.output())
	}
	added := koment(t, "add", "internal/serve.go", "--kind", "why", "--body", "Kept separate so main stays trivial.")
	if added.code != ExitOK {
		t.Fatalf("add exited %d: %s", added.code, added.output())
	}

	narrowed := koment(t, "check", "internal")
	if !strings.Contains(narrowed.stdout, "1 annotations across 1 files") {
		t.Errorf("check did not narrow to the given path:\n%s", narrowed.stdout)
	}

	everything := koment(t, "check")
	if !strings.Contains(everything.stdout, "2 annotations across 2 files") {
		t.Errorf("check without paths should cover everything:\n%s", everything.stdout)
	}
}

func TestAddRefusesAnExcerptThatIsNotThere(t *testing.T) {
	repository(t)

	got := koment(t, "add", "main.go", "--excerpt", "func absent()", "--kind", "why", "--body", "x")
	if got.code == ExitOK {
		t.Fatal("add should refuse an excerpt that does not appear in the file")
	}
	if !strings.Contains(got.stderr, "not found") {
		t.Errorf("want an explanation, got: %s", got.stderr)
	}
}

func TestAddRefusesAnAmbiguousExcerpt(t *testing.T) {
	root := repository(t)
	repeated := "package main\n\nfunc a() {\n\tserve()\n}\n\nfunc b() {\n\tserve()\n}\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(repeated), 0o644); err != nil {
		t.Fatal(err)
	}

	got := koment(t, "add", "main.go", "--excerpt", "\tserve()", "--kind", "why", "--body", "x")
	if got.code == ExitOK {
		t.Fatal("add should refuse an excerpt that matches more than one place")
	}
	if !strings.Contains(got.stderr, "matches 2 places") {
		t.Errorf("want the ambiguity explained, got: %s", got.stderr)
	}
}

func TestAddRejectsAnUnknownKind(t *testing.T) {
	repository(t)

	got := koment(t, "add", "main.go", "--kind", "todo", "--body", "x")
	if got.code != ExitUsage {
		t.Fatalf("want exit %d, got %d: %s", ExitUsage, got.code, got.output())
	}
	if !strings.Contains(got.stderr, "anti-pattern") {
		t.Errorf("the error should list the valid kinds, got: %s", got.stderr)
	}
}

func TestAddWithoutAnExcerptAnnotatesTheWholeFile(t *testing.T) {
	repository(t)

	added := koment(t, "add", "main.go", "--kind", "why", "--body", "Entry point only; logic lives in internal.")
	if added.code != ExitOK {
		t.Fatalf("add exited %d: %s", added.code, added.output())
	}

	shown := koment(t, "show", "main.go")
	if !strings.Contains(shown.stdout, "ok") || strings.Contains(shown.stdout, "main.go:") {
		t.Errorf("a file-scoped annotation should resolve ok with no line:\n%s", shown.stdout)
	}
}

func TestListFiltersByKind(t *testing.T) {
	repository(t)
	if got := addServeAnnotation(t); got.code != ExitOK {
		t.Fatalf("add exited %d: %s", got.code, got.output())
	}
	added := koment(t, "add", "main.go", "--kind", "why", "--body", "Entry point only.")
	if added.code != ExitOK {
		t.Fatalf("add exited %d: %s", added.code, added.output())
	}

	all := koment(t, "list")
	if !strings.Contains(all.stdout, "2 annotations") {
		t.Errorf("want both annotations listed:\n%s", all.stdout)
	}

	filtered := koment(t, "list", "--kind", "why")
	if !strings.Contains(filtered.stdout, "1 annotations") {
		t.Errorf("want one annotation after filtering:\n%s", filtered.stdout)
	}
	if strings.Contains(filtered.stdout, "signalled") {
		t.Errorf("the invariant should have been filtered out:\n%s", filtered.stdout)
	}
}

func TestShowReportsAnUnannotatedFilePlainly(t *testing.T) {
	repository(t)

	got := koment(t, "show", "main.go")
	if got.code != ExitOK {
		t.Fatalf("want exit %d, got %d: %s", ExitOK, got.code, got.output())
	}
	if !strings.Contains(got.stdout, "no annotations") {
		t.Errorf("want a plain statement, got:\n%s", got.stdout)
	}
}

func TestAddReadsTheBodyFromStdin(t *testing.T) {
	repository(t)

	var stdout, stderr bytes.Buffer
	env := Environment{
		Stdin:  strings.NewReader("Piped rationale that would be painful to quote in a shell."),
		Stdout: &stdout,
		Stderr: &stderr,
	}
	code := Run([]string{"add", "main.go", "--kind", "why", "--body", "-"}, env, Servers{})
	if code != ExitOK {
		t.Fatalf("add exited %d: %s%s", code, stdout.String(), stderr.String())
	}

	shown := koment(t, "show", "main.go")
	if !strings.Contains(shown.stdout, "Piped rationale") {
		t.Errorf("the piped body did not survive:\n%s", shown.stdout)
	}
}

func TestUnknownCommandIsAUsageError(t *testing.T) {
	repository(t)

	got := koment(t, "frobnicate")
	if got.code != ExitUsage {
		t.Fatalf("want exit %d, got %d", ExitUsage, got.code)
	}
	if !strings.Contains(got.stderr, "unknown command") {
		t.Errorf("want the command named, got: %s", got.stderr)
	}
}
