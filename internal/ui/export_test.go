package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func exportTo(t *testing.T) string {
	t.Helper()
	out := t.TempDir()
	taken := &snapshot{
		Commit:     "abc1234",
		CommitURL:  "https://example.test/koment/commit/abc1234",
		Banner:     "rebuilt nightly",
		BannerHref: "https://example.test/koment",
	}
	if _, err := export(annotatedRepository(t), out, "fixture", taken); err != nil {
		t.Fatalf("export: %v", err)
	}
	return out
}

func read(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestExportWritesAnIndexAndAPagePerFile(t *testing.T) {
	out := exportTo(t)

	for _, page := range []string{
		"index.html", "style.css", "koment-logo.svg", "koment-logo.png",
		filepath.Join("f", "main.go.html"),
	} {
		if _, err := os.Stat(filepath.Join(out, page)); err != nil {
			t.Errorf("missing %s: %v", page, err)
		}
	}

	index := read(t, filepath.Join(out, "index.html"))
	if !strings.Contains(index, rationale) {
		t.Error("the index should show the first file's annotations")
	}
}

func TestExportedLinksAreRelativeToTheOutputRoot(t *testing.T) {
	out := exportTo(t)

	index := read(t, filepath.Join(out, "index.html"))
	if !strings.Contains(index, `href="f/main.go.html"`) {
		t.Errorf("index link is not relative:\n%s", firstLines(index, 40))
	}
	if !strings.Contains(index, `href="style.css"`) {
		t.Error("index stylesheet link is not relative")
	}
	if !strings.Contains(index, `href="koment-logo.png"`) || !strings.Contains(index, `src="koment-logo.svg"`) {
		t.Error("index branding links are not relative")
	}

	page := read(t, filepath.Join(out, "f", "main.go.html"))
	if !strings.Contains(page, `href="../style.css"`) {
		t.Errorf("nested page does not walk back up to the stylesheet:\n%s", firstLines(page, 12))
	}
	if !strings.Contains(page, `href="../koment-logo.png"`) || !strings.Contains(page, `src="../koment-logo.svg"`) {
		t.Error("nested branding links do not walk back up to the output root")
	}
	if strings.Contains(page, `href="/f/`) || strings.Contains(page, `href="/assets/`) {
		t.Error("exported pages must not use absolute paths; they break under a subpath")
	}
}

func TestExportedLinksWalkUpForDeepPaths(t *testing.T) {
	how := exportedLinks("f/internal/store/ulid.go.html")

	if got, want := how.stylesheet, "../../../style.css"; got != want {
		t.Errorf("stylesheet: want %q, got %q", got, want)
	}
	if got, want := how.logoSVG, "../../../koment-logo.svg"; got != want {
		t.Errorf("logo: want %q, got %q", got, want)
	}
	if got, want := how.file("internal/cli/cli.go"), "../../../f/internal/cli/cli.go.html"; got != want {
		t.Errorf("file link: want %q, got %q", got, want)
	}
}

func TestExportedPagesNameTheirRepository(t *testing.T) {
	out := exportTo(t)

	for _, page := range []string{"index.html", filepath.Join("f", "main.go.html")} {
		content := read(t, filepath.Join(out, page))
		if !strings.Contains(content, "fixture") {
			t.Errorf("%s does not say which repository it belongs to", page)
		}
	}
}

// A published page is a snapshot, and a snapshot that does not name its commit
// is how a stale rendering passes for a current one (ADR 0026).
func TestEveryPublishedPageNamesItsCommit(t *testing.T) {
	out := exportTo(t)

	for _, page := range []string{"index.html", filepath.Join("f", "main.go.html")} {
		content := read(t, filepath.Join(out, page))
		if !strings.Contains(content, "abc1234") {
			t.Errorf("%s does not name the commit it renders", page)
		}
		if !strings.Contains(content, "snapshot of") {
			t.Errorf("%s does not admit to being a snapshot", page)
		}
		if !strings.Contains(content, "rebuilt nightly") {
			t.Errorf("%s dropped the publisher's banner", page)
		}
	}
}

func TestPublishingRefusesWithoutACommit(t *testing.T) {
	outside := t.TempDir()

	if _, err := commitOf(outside, ""); err == nil {
		t.Fatal("want a refusal when git cannot name the commit")
	}
	got, err := commitOf(outside, "deadbee")
	if err != nil || got != "deadbee" {
		t.Errorf("an explicit --commit should be taken as given, got %q %v", got, err)
	}
}

// A site rendered from a modified tree is not the commit it sits on, and saying
// so plainly is cheaper than a reader discovering it later.
func TestASiteRenderedFromAModifiedTreeSaysSo(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = root
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.test")
	run("config", "user.name", "test")
	run("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "main.go")
	run("commit", "-qm", "first")

	clean, err := commitOf(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(clean, "-dirty") {
		t.Fatalf("a clean tree should name its commit plainly, got %q", clean)
	}

	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main // edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, err := commitOf(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dirty, "-dirty") {
		t.Errorf("a modified tree must not claim to be its commit, got %q", dirty)
	}
}

func TestExportRendersTheSameContentAsTheServer(t *testing.T) {
	annotations := annotatedRepository(t)
	out := t.TempDir()
	if _, err := export(annotations, out, "fixture", &snapshot{Commit: "abc1234"}); err != nil {
		t.Fatal(err)
	}

	_, served := get(t, annotations, "/f/main.go")
	exported := read(t, filepath.Join(out, "f", "main.go.html"))

	for _, want := range []string{rationale, "func main()", "invariant", `class="row marked`} {
		if !strings.Contains(served, want) {
			t.Errorf("served page missing %q", want)
		}
		if !strings.Contains(exported, want) {
			t.Errorf("exported page missing %q", want)
		}
	}
}

func TestSiteNeedsAnOutputDirectory(t *testing.T) {
	var stderr strings.Builder
	if err := Site(nil, &stderr); err == nil {
		t.Fatal("want an error without --out")
	}
}
