package ui

import (
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	if _, err := export(snapshotFromStore(t, annotatedRepository(t)), out, "fixture", taken, nil); err != nil {
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
	if !strings.Contains(index, `href="https://github.com/janpuc/koment"`) {
		t.Error("the project source should remain reachable from the bottom of the rail")
	}
	for _, want := range []string{`data-search-dialog`, `data-search-open hidden`, `data-path="main.go"`} {
		if !strings.Contains(index, want) {
			t.Errorf("the exported search shell is missing %s", want)
		}
	}
}

func TestPublishedPathsCarryNoDotComponent(t *testing.T) {
	for source, want := range map[string]string{
		".github/workflows/ci.yml": "f/dot-github/workflows/ci.yml.html",
		".mise/config.toml":        "f/dot-mise/config.toml.html",
		"internal/store/record.go": "f/internal/store/record.go.html",
		"a/.hidden/b.go":           "f/a/dot-hidden/b.go.html",
	} {
		if got := publishedPagePath(source); got != want {
			t.Errorf("publishedPagePath(%q) = %q, want %q", source, got, want)
		}
	}
}

// A page nothing links to is a page nobody reaches, so the written path and
// the link have to come from the same function.
func TestEveryLinkedPageWasWritten(t *testing.T) {
	out := exportTo(t)
	index := read(t, filepath.Join(out, indexPage))
	for _, match := range regexp.MustCompile(`href="(f/[^"]+)"`).FindAllStringSubmatch(index, -1) {
		unescaped, err := url.PathUnescape(match[1])
		if err != nil {
			t.Fatalf("link %q is not a valid path: %v", match[1], err)
		}
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(unescaped))); err != nil {
			t.Errorf("index links %q but no page was written there", match[1])
		}
	}
}

func TestExportWritesMachineReadableSnapshotAndBodySearch(t *testing.T) {
	out := exportTo(t)
	var published staticPublication
	if err := json.Unmarshal([]byte(read(t, filepath.Join(out, annotationsName))), &published); err != nil {
		t.Fatal(err)
	}
	if published.Version != 1 || published.Repository.Commit != "abc1234" || len(published.Files) != 1 {
		t.Fatalf("published = %#v", published)
	}
	if len(published.Files[0].Annotations) != 1 || published.Files[0].Annotations[0].Body != rationale {
		t.Fatalf("annotations = %#v", published.Files[0].Annotations)
	}
	var searchable []searchEntry
	if err := json.Unmarshal([]byte(read(t, filepath.Join(out, searchName))), &searchable); err != nil {
		t.Fatal(err)
	}
	if len(searchable) != 1 || searchable[0].Body != rationale {
		t.Fatalf("search = %#v", searchable)
	}
	index := read(t, filepath.Join(out, indexPage))
	if !strings.Contains(index, "serve must be the last call") || !strings.Contains(index, "data-search=") {
		t.Fatalf("index has no annotation-body search data:\n%s", firstLines(index, 60))
	}
}

func TestPublishReplacesThePreviousSnapshot(t *testing.T) {
	annotations := annotatedRepository(t)
	out := filepath.Join(t.TempDir(), "site")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "stale.html"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := publish(snapshotFromStore(t, annotations), out, annotations.Root(), "fixture", &snapshot{Commit: "abc1234"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "stale.html")); !os.IsNotExist(err) {
		t.Fatalf("stale output remains: %v", err)
	}
	for _, name := range []string{indexPage, annotationsName, searchName} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestPublishRefusesToReplaceTheRepositoryRoot(t *testing.T) {
	annotations := annotatedRepository(t)
	_, err := publish(snapshotFromStore(t, annotations), annotations.Root(), annotations.Root(), "fixture", &snapshot{Commit: "abc1234"}, nil)
	if err == nil || !strings.Contains(err.Error(), "unsafe output") {
		t.Fatalf("err = %v", err)
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

func TestExportedPagesKeepRepositorySwitchingInContext(t *testing.T) {
	annotations := annotatedRepository(t)
	out := t.TempDir()
	repositories := []repositoryLink{
		{Name: "koment", Href: "index.html", Current: true},
		{Name: "workspace", Href: "r/workspace/index.html"},
	}
	if _, err := export(snapshotFromStore(t, annotations), out, "koment", &snapshot{Commit: "abc1234"}, repositories); err != nil {
		t.Fatal(err)
	}

	index := read(t, filepath.Join(out, "index.html"))
	if !strings.Contains(index, `value="r/workspace/index.html"`) ||
		!strings.Contains(index, `value="index.html" selected`) {
		t.Errorf("root page has no contextual switcher:\n%s", firstLines(index, 45))
	}
	file := read(t, filepath.Join(out, "f", "main.go.html"))
	if !strings.Contains(file, `value="../r/workspace/index.html"`) {
		t.Errorf("file page does not preserve the switch target:\n%s", firstLines(file, 45))
	}
}

func TestRepositoryLinkConfigurationRequiresTheCurrentRepository(t *testing.T) {
	for _, specification := range []string{"one=index.html", "koment,workspace=other", "other=a,workspace=b"} {
		if _, err := parseRepositoryLinks(specification, "koment"); err == nil {
			t.Errorf("%q should be rejected", specification)
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
	if _, err := export(snapshotFromStore(t, annotations), out, "fixture", &snapshot{Commit: "abc1234"}, nil); err != nil {
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
