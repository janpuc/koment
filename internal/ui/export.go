package ui

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	flags.Usage = func() { fmt.Fprint(stderr, exportUsage) }

	out := flags.String("out", "", "directory to write into")
	banner := flags.String("banner", "", "notice shown on every page, naming the snapshot")
	bannerHref := flags.String("banner-link", "", "URL shown beside the banner")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("export needs --out")
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("finding the working directory: %w", err)
	}
	root, err := store.FindRoot(workingDirectory)
	if err != nil {
		return err
	}

	written, err := export(store.Open(root), *out, *banner, *bannerHref)
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "koment: wrote %d pages to %s\n", written, *out)
	return nil
}

func export(annotations *store.Store, out, banner, bannerHref string) (int, error) {
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
		rendered, err := renderPage(templates, annotations, file, exportedLinks(page), banner, bannerHref)
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

func renderPage(templates *template.Template, annotations *store.Store, file string, how links, banner, bannerHref string) ([]byte, error) {
	built, err := build(annotations, file, how)
	if err != nil {
		return nil, err
	}
	built.Banner = banner
	built.BannerHref = bannerHref

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
