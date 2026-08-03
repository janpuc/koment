package ui

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/janpuc/koment/internal/config"
	"github.com/janpuc/koment/internal/provenance"
	"github.com/janpuc/koment/internal/store"
)

const (
	exportedSuffix = ".html"
	indexPage      = "index.html"
	stylesheetName = "style.css"
	scriptName     = "koment.js"
)

const exportUsage = `koment site renders one repository to static HTML.

  koment site --out <dir> [--banner <text>]

This is the published tier (ADR 0026): everyone reads the annotations in a
browser, with no server to run and no authentication to design. Point it at a
directory, commit a workflow, and GitHub Pages serves it — see docs/publishing.md.

It renders a snapshot of one commit rather than your working tree, and every
page says which commit. Read your own tree with koment ui instead, which
re-resolves on every request.

A site renders your source as well as your annotations. Publishing one from a
private repository publishes that source.
`

// Export writes the same pages koment ui serves, with relative links so the
// tree survives being hosted under a subpath.
func Site(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("site", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(stderr, exportUsage, "\nFlags (each also settable from the environment):\n", config.Usage(flags))
	}

	out := flags.String("out", "", "directory to write into")
	name := flags.String("name", "", "repository name shown on every page; defaults to the repository's own name")
	named := flags.String("repository", "", "which repository to render; required when several are configured")
	commit := flags.String("commit", "", "commit this snapshot renders; read from git when omitted")
	commitURL := flags.String("commit-link", "", "URL the commit links to")
	banner := flags.String("banner", "", "notice shown on every page, beside the commit")
	bannerHref := flags.String("banner-link", "", "URL shown beside the banner")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := config.FromEnvironment(flags); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("site needs --out")
	}

	repositories, err := serveable(*named)
	if err != nil {
		return err
	}
	chosen, single := repositories.Only()
	if !single {
		return fmt.Errorf("%d repositories are configured (%s); a site renders one, so pass --repository",
			repositories.Len(), strings.Join(repositories.IDs(), ", "))
	}

	taken := &snapshot{
		Commit:     *commit,
		CommitURL:  *commitURL,
		Banner:     *banner,
		BannerHref: *bannerHref,
	}
	if taken.Commit, err = commitOf(chosen.Root, *commit); err != nil {
		return err
	}

	label := *name
	if label == "" {
		label = chosen.Display()
	}
	written, err := export(chosen.Store(), *out, label, taken)
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "koment: wrote %d pages to %s at %s\n", written, *out, taken.Commit)
	return nil
}

// commitOf refuses to publish a snapshot that cannot say what it is a snapshot
// of. An undated page is how a reader mistakes an old rendering for the current
// one, which is the failure this project exists to prevent (ADR 0026).
func commitOf(root, given string) (string, error) {
	if given != "" {
		return given, nil
	}
	commit, err := provenance.HeadCommit(root)
	if err != nil {
		return "", fmt.Errorf("cannot read the commit at %s: every published page names the commit it renders; pass --commit", root)
	}
	if provenance.TreeIsDirty(root) {
		return commit + "-dirty", nil
	}
	return commit, nil
}

func export(annotations *store.Store, out, name string, taken *snapshot) (int, error) {
	files, err := annotations.AnnotatedFiles()
	if err != nil {
		return 0, err
	}
	templates := template.Must(template.ParseFS(assets, "assets/*.html"))

	for _, asset := range []string{stylesheetName, scriptName} {
		content, err := assets.ReadFile("assets/" + asset)
		if err != nil {
			return 0, err
		}
		if err := writeFile(filepath.Join(out, asset), content); err != nil {
			return 0, err
		}
	}

	pages := map[string]string{indexPage: ""}
	for _, file := range files {
		pages[filepath.ToSlash(filepath.Join("f", file+exportedSuffix))] = file
	}

	for page, file := range pages {
		rendered, err := renderPage(templates, annotations, file, exportedLinks(page), name, taken)
		if err != nil {
			return 0, err
		}
		if err := writeFile(filepath.Join(out, filepath.FromSlash(page)), rendered); err != nil {
			return 0, err
		}
	}
	return len(pages), nil
}

// exportedLinks walks back up to the output root from wherever this page sits,
// so the tree works from a file:// path and from a project subpath alike.
func exportedLinks(page string) links {
	up := strings.Repeat("../", strings.Count(page, "/"))
	return links{
		file:       func(target string) string { return up + "f/" + target + exportedSuffix },
		home:       up + indexPage,
		stylesheet: up + stylesheetName,
		script:     up + scriptName,
	}
}

func renderPage(templates *template.Template, annotations *store.Store, file string, how links, name string, taken *snapshot) ([]byte, error) {
	built, err := build(annotations, file, how)
	if err != nil {
		return nil, err
	}
	built.Repository = name
	built.Snapshot = taken

	var page strings.Builder
	if err := templates.ExecuteTemplate(&page, "page.html", built); err != nil {
		return nil, err
	}
	return []byte(page.String()), nil
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
