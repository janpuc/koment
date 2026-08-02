package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func exportTo(t *testing.T) string {
	t.Helper()
	out := t.TempDir()
	if _, err := export(repository(t), out, "snapshot of commit abc1234", "https://example.test/koment", "fixture"); err != nil {
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

	for _, page := range []string{"index.html", "style.css", filepath.Join("f", "main.go.html")} {
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

	page := read(t, filepath.Join(out, "f", "main.go.html"))
	if !strings.Contains(page, `href="../style.css"`) {
		t.Errorf("nested page does not walk back up to the stylesheet:\n%s", firstLines(page, 12))
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

func TestExportedPagesCarryTheSnapshotBanner(t *testing.T) {
	out := exportTo(t)

	for _, page := range []string{"index.html", filepath.Join("f", "main.go.html")} {
		content := read(t, filepath.Join(out, page))
		if !strings.Contains(content, "snapshot of commit abc1234") {
			t.Errorf("%s does not say it is a snapshot", page)
		}
	}
}

func TestExportRendersTheSameContentAsTheServer(t *testing.T) {
	annotations := repository(t)
	out := t.TempDir()
	if _, err := export(annotations, out, "", "", "fixture"); err != nil {
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
