package ui

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/config"
	"github.com/janpuc/koment/internal/provenance"
)

const (
	exportedSuffix  = ".html"
	indexPage       = "index.html"
	stylesheetName  = "style.css"
	scriptName      = "koment.js"
	logoSVGName     = "koment-logo.svg"
	logoPNGName     = "koment-logo.png"
	annotationsName = "annotations.json"
	searchName      = "search.json"
)

const exportUsage = `koment site renders a repository snapshot to static HTML.

  koment site --out <dir> [--banner <text>]

This is the published tier (ADR 0103): everyone reads the annotations in a
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
	repositoryLinks := flags.String("repository-links", "", "comma-separated name=URL entries for the contextual repository switcher")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := config.FromEnvironment(flags); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("site needs --out")
	}

	repositories, err := selectedRepositories(*named)
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
	linked, err := parseRepositoryLinks(*repositoryLinks, label)
	if err != nil {
		return err
	}
	repositorySnapshot, err := application.BuildSnapshot(chosen)
	if err != nil {
		return err
	}
	written, err := publish(repositorySnapshot, *out, chosen.Root, label, taken, linked)
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "koment: wrote %d pages to %s at %s\n", written, *out, taken.Commit)
	return nil
}

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

func export(repositorySnapshot *application.RepositorySnapshot, out, name string, taken *snapshot, repositories []repositoryLink) (int, error) {
	templates := template.Must(template.ParseFS(assets, "assets/*.html"))

	for _, asset := range []string{stylesheetName, scriptName, logoSVGName, logoPNGName} {
		content, err := assets.ReadFile("assets/" + asset)
		if err != nil {
			return 0, err
		}
		if err := writeFile(filepath.Join(out, asset), content); err != nil {
			return 0, err
		}
	}

	pages := map[string]string{indexPage: ""}
	for _, file := range repositorySnapshot.Files {
		pages[filepath.ToSlash(filepath.Join("f", file.Path+exportedSuffix))] = file.Path
	}

	for page, file := range pages {
		rendered, err := renderPage(templates, repositorySnapshot, file, exportedLinks(page), name, taken,
			exportedRepositoryLinks(page, repositories))
		if err != nil {
			return 0, err
		}
		if err := writeFile(filepath.Join(out, filepath.FromSlash(page)), rendered); err != nil {
			return 0, err
		}
	}
	if err := writeJSON(filepath.Join(out, annotationsName), staticData(repositorySnapshot, name, taken)); err != nil {
		return 0, err
	}
	if err := writeJSON(filepath.Join(out, searchName), searchData(repositorySnapshot)); err != nil {
		return 0, err
	}
	return len(pages), nil
}

func exportedLinks(page string) links {
	up := strings.Repeat("../", strings.Count(page, "/"))
	return links{
		file:       func(target string) string { return up + "f/" + escapedFilePath(target) + exportedSuffix },
		home:       up + indexPage,
		stylesheet: up + stylesheetName,
		script:     up + scriptName,
		logoSVG:    up + logoSVGName,
		logoPNG:    up + logoPNGName,
	}
}

func renderPage(templates *template.Template, repositorySnapshot *application.RepositorySnapshot, file string, how links, name string,
	taken *snapshot, repositories []repositoryLink,
) ([]byte, error) {
	built, err := build(repositorySnapshot, file, how)
	if err != nil {
		return nil, err
	}
	built.Repository = name
	built.Snapshot = taken
	built.Repositories = repositories

	var page strings.Builder
	if err := templates.ExecuteTemplate(&page, "page.html", built); err != nil {
		return nil, err
	}
	return []byte(page.String()), nil
}

func parseRepositoryLinks(specification, current string) ([]repositoryLink, error) {
	if strings.TrimSpace(specification) == "" {
		return nil, nil
	}
	var links []repositoryLink
	currentCount := 0
	for _, entry := range strings.Split(specification, ",") {
		name, target, found := strings.Cut(entry, "=")
		name = strings.TrimSpace(name)
		target = strings.TrimSpace(target)
		if !found || name == "" || target == "" {
			return nil, fmt.Errorf("repository-links entry %q must be name=URL", entry)
		}
		isCurrent := name == current
		if isCurrent {
			currentCount++
		}
		links = append(links, repositoryLink{Name: name, Href: target, Current: isCurrent})
	}
	if len(links) < 2 {
		return nil, fmt.Errorf("repository-links needs at least two entries")
	}
	if currentCount != 1 {
		return nil, fmt.Errorf("repository-links must contain the current repository %q exactly once", current)
	}
	return links, nil
}

func exportedRepositoryLinks(page string, repositories []repositoryLink) []repositoryLink {
	if len(repositories) == 0 {
		return nil
	}
	up := strings.Repeat("../", strings.Count(page, "/"))
	linked := make([]repositoryLink, len(repositories))
	for index, repository := range repositories {
		linked[index] = repository
		if !strings.Contains(repository.Href, "://") && !strings.HasPrefix(repository.Href, "/") {
			linked[index].Href = up + repository.Href
		}
	}
	return linked
}

type staticPublication struct {
	Version     int              `json:"version"`
	Repository  staticRepository `json:"repository"`
	GeneratedAt string           `json:"generated_at"`
	Files       []staticFile     `json:"files"`
}

type staticRepository struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Commit        string `json:"commit"`
	CloneURL      string `json:"clone_url,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

type staticFile struct {
	Path        string             `json:"path"`
	Exists      bool               `json:"exists"`
	Source      string             `json:"source,omitempty"`
	Annotations []staticAnnotation `json:"annotations"`
}

type staticAnnotation struct {
	ID          string        `json:"id"`
	Kind        string        `json:"kind"`
	Title       string        `json:"title"`
	Body        string        `json:"body"`
	Created     string        `json:"created"`
	Status      string        `json:"status"`
	Line        int           `json:"line,omitempty"`
	Occurrences int           `json:"occurrences"`
	Warning     string        `json:"warning,omitempty"`
	Anchor      staticAnchor  `json:"anchor"`
	Author      staticAuthor  `json:"author"`
	Git         *staticGit    `json:"git,omitempty"`
	Policy      *staticPolicy `json:"policy,omitempty"`
}

type staticAnchor struct {
	Scope        string `json:"scope"`
	Excerpt      string `json:"excerpt,omitempty"`
	Before       string `json:"before,omitempty"`
	After        string `json:"after,omitempty"`
	LastSeenLine int    `json:"last_seen_line,omitempty"`
}

type staticAuthor struct {
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Kind     string `json:"kind"`
	Source   string `json:"source"`
	Account  string `json:"account,omitempty"`
	Verified string `json:"verified,omitempty"`
}

type staticGit struct {
	Commit  string `json:"commit"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	EndLine int    `json:"end_line,omitempty"`
}

type staticPolicy struct {
	Exception    string `json:"exception"`
	Acknowledged bool   `json:"acknowledged"`
}

type searchEntry struct {
	File    string `json:"file"`
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Author  string `json:"author"`
	Status  string `json:"status"`
	Warning string `json:"warning,omitempty"`
	Line    int    `json:"line,omitempty"`
}

func staticData(repositorySnapshot *application.RepositorySnapshot, name string, taken *snapshot) staticPublication {
	published := staticPublication{
		Version: 1,
		Repository: staticRepository{
			ID: repositorySnapshot.Repository.ID, Name: name,
			Commit: taken.Commit, CloneURL: repositorySnapshot.Repository.CloneURL,
			DefaultBranch: repositorySnapshot.Repository.DefaultBranch,
		},
		GeneratedAt: repositorySnapshot.GeneratedAt.Format(time.RFC3339Nano),
	}
	for _, file := range repositorySnapshot.Files {
		publishedFile := staticFile{Path: file.Path, Exists: file.Exists, Source: string(file.Content)}
		for _, annotation := range file.Annotations {
			record := annotation.Record
			publishedAnnotation := staticAnnotation{
				ID: record.ID, Kind: string(record.Kind), Title: record.Headline(), Body: record.Body,
				Created: record.Created.Format("2006-01-02"), Status: string(annotation.Status),
				Line: annotation.Line, Occurrences: annotation.Occurrences, Warning: annotation.Warning,
				Anchor: staticAnchor{
					Scope: string(record.Anchor.Scope), Excerpt: record.Anchor.Excerpt,
					Before: record.Anchor.Before, After: record.Anchor.After, LastSeenLine: record.Anchor.LastSeenLine,
				},
				Author: staticAuthor{
					Name: record.Author.Name, Email: record.Author.Email, Kind: string(record.Author.Kind),
					Source: string(record.Author.Source), Account: record.Author.Account, Verified: record.Author.Verified,
				},
			}
			if record.Git != nil {
				publishedAnnotation.Git = &staticGit{
					Commit: record.Git.Commit, Path: record.Git.Path, Line: record.Git.Line, EndLine: record.Git.EndLine,
				}
			}
			if record.Policy != nil {
				publishedAnnotation.Policy = &staticPolicy{
					Exception: record.Policy.Exception, Acknowledged: record.Policy.Acknowledged,
				}
			}
			publishedFile.Annotations = append(publishedFile.Annotations, publishedAnnotation)
		}
		published.Files = append(published.Files, publishedFile)
	}
	return published
}

func searchData(repositorySnapshot *application.RepositorySnapshot) []searchEntry {
	var entries []searchEntry
	for _, file := range repositorySnapshot.Files {
		for _, annotation := range file.Annotations {
			entries = append(entries, searchEntry{
				File: file.Path, ID: annotation.Record.ID, Kind: string(annotation.Record.Kind),
				Title: annotation.Record.Headline(),
				Body:  annotation.Record.Body, Author: annotation.Record.Author.Name,
				Status: string(annotation.Status), Warning: annotation.Warning, Line: annotation.Line,
			})
		}
	}
	return entries
}

func writeJSON(name string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s: %w", name, err)
	}
	return writeFile(name, append(content, '\n'))
}

func publish(repositorySnapshot *application.RepositorySnapshot, out, repositoryRoot, name string, taken *snapshot,
	repositories []repositoryLink,
) (_ int, returnedError error) {
	absoluteOut, err := filepath.Abs(out)
	if err != nil {
		return 0, fmt.Errorf("resolving output directory %s: %w", out, err)
	}
	absoluteRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return 0, fmt.Errorf("resolving repository root %s: %w", repositoryRoot, err)
	}
	if filepath.Clean(absoluteOut) == filepath.Clean(string(filepath.Separator)) || filepath.Clean(absoluteOut) == filepath.Clean(absoluteRoot) {
		return 0, fmt.Errorf("refusing to replace unsafe output directory %s", absoluteOut)
	}
	parent := filepath.Dir(absoluteOut)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return 0, fmt.Errorf("creating output parent %s: %w", parent, err)
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(absoluteOut)+".staging-")
	if err != nil {
		return 0, fmt.Errorf("creating staging directory beside %s: %w", absoluteOut, err)
	}
	defer func() {
		if staging != "" {
			returnedError = errors.Join(returnedError, os.RemoveAll(staging))
		}
	}()
	written, err := export(repositorySnapshot, staging, name, taken, repositories)
	if err != nil {
		return 0, err
	}
	if err := replaceDirectory(staging, absoluteOut); err != nil {
		return 0, err
	}
	staging = ""
	return written, nil
}

func replaceDirectory(staging, destination string) error {
	information, err := os.Stat(destination)
	if errors.Is(err, os.ErrNotExist) {
		return os.Rename(staging, destination)
	}
	if err != nil {
		return fmt.Errorf("inspecting output directory %s: %w", destination, err)
	}
	if !information.IsDir() {
		return fmt.Errorf("output path %s is not a directory", destination)
	}
	parent := filepath.Dir(destination)
	backup, err := os.MkdirTemp(parent, "."+filepath.Base(destination)+".previous-")
	if err != nil {
		return fmt.Errorf("reserving backup beside %s: %w", destination, err)
	}
	if err := os.Remove(backup); err != nil {
		return fmt.Errorf("preparing backup path %s: %w", backup, err)
	}
	if err := os.Rename(destination, backup); err != nil {
		return fmt.Errorf("moving previous output %s aside: %w", destination, err)
	}
	if err := os.Rename(staging, destination); err != nil {
		return errors.Join(fmt.Errorf("publishing output %s: %w", destination, err), os.Rename(backup, destination))
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("removing previous output %s: %w", backup, err)
	}
	return nil
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
