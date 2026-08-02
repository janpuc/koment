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
	"github.com/janpuc/koment/internal/store"
)

const (
	exportedSuffix = ".html"
	indexPage      = "index.html"
	stylesheetName = "style.css"
)

const exportUsage = `koment export renders the view to static HTML.

  koment export --out <dir> [--banner <text>]

This exists to publish koment's own demo site (ADR 0014). It is a snapshot, not
a view of your working tree: read your own annotations with koment ui, which
re-reads the tree on every request. Exporting a private repository and hosting
the result publishes your source and your annotations.
`

// Export writes the same pages koment ui serves, with relative links so the
// tree survives being hosted under a subpath.
func Export(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(stderr, exportUsage, "\nFlags (each also settable from the environment):\n", config.Usage(flags))
	}

	out := flags.String("out", "", "directory to write into")
	name := flags.String("name", "", "repository name shown on every page; defaults to the directory name")
	banner := flags.String("banner", "", "notice shown on every page, naming the snapshot")
	bannerHref := flags.String("banner-link", "", "URL shown beside the banner")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := config.FromEnvironment(flags); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("export needs --out")
	}

	root, err := repositoryRoot()
	if err != nil {
		return err
	}

	written, err := export(store.Open(root), *out, *banner, *bannerHref, repositoryName(*name, root))
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "koment: wrote %d pages to %s\n", written, *out)
	return nil
}

func repositoryName(given, root string) string {
	if given != "" {
		return given
	}
	return filepath.Base(root)
}

func export(annotations *store.Store, out, banner, bannerHref, name string) (int, error) {
	files, err := annotations.AnnotatedFiles()
	if err != nil {
		return 0, err
	}
	templates := template.Must(template.ParseFS(assets, "assets/*.html"))

	stylesheet, err := assets.ReadFile("assets/" + stylesheetName)
	if err != nil {
		return 0, err
	}
	if err := writeFile(filepath.Join(out, stylesheetName), stylesheet); err != nil {
		return 0, err
	}

	pages := map[string]string{indexPage: ""}
	for _, file := range files {
		pages[filepath.ToSlash(filepath.Join("f", file+exportedSuffix))] = file
	}

	for page, file := range pages {
		rendered, err := renderPage(templates, annotations, file, exportedLinks(page), banner, bannerHref, name)
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
		stylesheet: up + stylesheetName,
	}
}

func renderPage(templates *template.Template, annotations *store.Store, file string, how links, banner, bannerHref, name string) ([]byte, error) {
	built, err := build(annotations, file, how)
	if err != nil {
		return nil, err
	}
	built.Banner = banner
	built.BannerHref = bannerHref
	built.Repository = name

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
