package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func annotationID(t *testing.T, addOutput string) string {
	t.Helper()
	fields := strings.Fields(addOutput)
	if len(fields) == 0 {
		t.Fatalf("no id in add output: %q", addOutput)
	}
	return fields[0]
}

func TestReanchorFixesDriftAndKeepsTheID(t *testing.T) {
	root := repository(t)
	added := addServeAnnotation(t)
	if added.code != ExitOK {
		t.Fatalf("add exited %d: %s", added.code, added.output())
	}
	id := annotationID(t, added.stdout)

	edited := strings.Replace(source, "\tserve()", "\tserveForever()", 1)
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if drifted := koment(t, "check"); drifted.code != ExitFailure {
		t.Fatalf("expected drift, got exit %d", drifted.code)
	}

	fixed := koment(t, "reanchor", id, "--excerpt", "\tserveForever()")
	if fixed.code != ExitOK {
		t.Fatalf("reanchor exited %d: %s", fixed.code, fixed.output())
	}
	if !strings.Contains(fixed.stdout, id) {
		t.Errorf("reanchor must keep the id %s, got: %s", id, fixed.stdout)
	}

	green := koment(t, "check")
	if green.code != ExitOK {
		t.Fatalf("check still failing after reanchor: %s", green.output())
	}
	if !strings.Contains(green.stdout, "1 ok") {
		t.Errorf("want the annotation resolving again:\n%s", green.stdout)
	}
}

func TestReanchorRecapturesTheExcerptAndLine(t *testing.T) {
	root := repository(t)
	added := addServeAnnotation(t)
	id := annotationID(t, added.stdout)

	edited := strings.Replace(source, "\tserve()", "\tserveForever()", 1)
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if fixed := koment(t, "reanchor", id, "--excerpt", "\tserveForever()"); fixed.code != ExitOK {
		t.Fatalf("reanchor exited %d: %s", fixed.code, fixed.output())
	}

	record, err := os.ReadFile(filepath.Join(root, ".koment", "annotations", id+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(record)
	if !strings.Contains(text, "serveForever") {
		t.Errorf("excerpt was not updated:\n%s", text)
	}
	if strings.Contains(text, "excerpt: \"\\tserve()\"") {
		t.Errorf("old excerpt still present:\n%s", text)
	}
	if !strings.Contains(text, "lastSeenLine: 4") {
		t.Errorf("last_seen_line was not recomputed:\n%s", text)
	}
}

func TestReanchorMovesAnOrphanToItsNewFile(t *testing.T) {
	root := repository(t)
	added := addServeAnnotation(t)
	id := annotationID(t, added.stdout)

	renamed := filepath.Join(root, "internal")
	if err := os.MkdirAll(renamed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(renamed, "serve.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	if orphaned := koment(t, "check"); orphaned.code != ExitFailure {
		t.Fatalf("expected an orphan, got exit %d", orphaned.code)
	}

	moved := koment(t, "reanchor", id, "--file", "internal/serve.go")
	if moved.code != ExitOK {
		t.Fatalf("reanchor exited %d: %s", moved.code, moved.output())
	}

	green := koment(t, "check")
	if green.code != ExitOK {
		t.Fatalf("check still failing after the move: %s", green.output())
	}
	if !strings.Contains(green.stdout, "across 1 files") {
		t.Errorf("the old record should be gone:\n%s", green.stdout)
	}
	record, err := os.ReadFile(filepath.Join(root, ".koment", "annotations", id+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(record), "file: internal/serve.go") {
		t.Errorf("the one authoritative record did not move to the new source path:\n%s", record)
	}
}

func TestReanchorRefusesAnExcerptThatIsNotThere(t *testing.T) {
	repository(t)
	added := addServeAnnotation(t)
	id := annotationID(t, added.stdout)

	got := koment(t, "reanchor", id, "--excerpt", "func nowhere()")
	if got.code == ExitOK {
		t.Fatal("reanchor should refuse an excerpt that does not appear")
	}
	if !strings.Contains(got.stderr, "not found") {
		t.Errorf("want an explanation, got: %s", got.stderr)
	}
}

func TestReanchorRefusesAnAmbiguousExcerpt(t *testing.T) {
	root := repository(t)
	added := addServeAnnotation(t)
	id := annotationID(t, added.stdout)

	repeated := "package main\n\nfunc a() {\n\tboth()\n}\n\nfunc b() {\n\tboth()\n}\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(repeated), 0o644); err != nil {
		t.Fatal(err)
	}

	got := koment(t, "reanchor", id, "--excerpt", "\tboth()")
	if got.code == ExitOK {
		t.Fatal("reanchor should refuse an ambiguous excerpt, as add does")
	}
	if !strings.Contains(got.stderr, "matches 2 places") {
		t.Errorf("want the ambiguity explained, got: %s", got.stderr)
	}
}

func TestReanchorRejectsAnUnknownID(t *testing.T) {
	repository(t)

	got := koment(t, "reanchor", "01JQ8ZK3M4N5P6R7S8T9V0W1X2", "--excerpt", "x")
	if got.code == ExitOK {
		t.Fatal("want a failure for an id that does not exist")
	}
	if !strings.Contains(got.stderr, "no annotation with id") {
		t.Errorf("want a clear error, got: %s", got.stderr)
	}
}

func TestReanchorNeedsSomethingToDo(t *testing.T) {
	repository(t)
	added := addServeAnnotation(t)
	id := annotationID(t, added.stdout)

	got := koment(t, "reanchor", id)
	if got.code != ExitUsage {
		t.Fatalf("want exit %d, got %d: %s", ExitUsage, got.code, got.output())
	}
	if !strings.Contains(got.stderr, "--excerpt") {
		t.Errorf("want the options named, got: %s", got.stderr)
	}
}

func TestReanchorKeepsOtherAnnotationsInTheRecord(t *testing.T) {
	root := repository(t)
	added := addServeAnnotation(t)
	id := annotationID(t, added.stdout)
	if second := koment(t, "add", "main.go", "--kind", "why", "--body", "Entry point only."); second.code != ExitOK {
		t.Fatalf("add exited %d: %s", second.code, second.output())
	}

	other := filepath.Join(root, "other.go")
	if err := os.WriteFile(other, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	if moved := koment(t, "reanchor", id, "--file", "other.go"); moved.code != ExitOK {
		t.Fatalf("reanchor exited %d: %s", moved.code, moved.output())
	}

	green := koment(t, "check")
	if green.code != ExitOK {
		t.Fatalf("check failing: %s", green.output())
	}
	if !strings.Contains(green.stdout, "2 annotations across 2 files") {
		t.Errorf("the file-scoped annotation should have stayed behind:\n%s", green.stdout)
	}
}
