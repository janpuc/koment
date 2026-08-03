package ui

import (
	"net/url"
	"strings"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/store"
)

type view struct {
	Total        int
	Tally        []tallyEntry
	Tree         []treeNode
	Loose        []entry
	Repositories []repositoryLink
	Repository   string
	Current      string
	File         *fileView
	Empty        bool
	NotFound     bool
	Home         string
	Stylesheet   string
	Script       string
	LogoSVG      string
	LogoPNG      string
	Snapshot     *snapshot
	WriteToken   string
	CreatedID    string
	WriteWarning string
}

type snapshot struct {
	Commit     string
	CommitURL  string
	Banner     string
	BannerHref string
}

type repositoryLink struct {
	ID      string
	Name    string
	Href    string
	Current bool
}

type links struct {
	file       func(target string) string
	home       string
	stylesheet string
	script     string
	logoSVG    string
	logoPNG    string
}

func servedLinks(repositoryID string) links {
	base := repositoryPrefix + repositoryID + "/"
	return links{
		file:       func(target string) string { return base + "f/" + escapedFilePath(target) },
		home:       base,
		stylesheet: "/assets/style.css",
		script:     "/assets/koment.js",
		logoSVG:    "/assets/koment-logo.svg",
		logoPNG:    "/assets/koment-logo.png",
	}
}

func escapedFilePath(file string) string {
	parts := strings.Split(file, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

type tallyEntry struct {
	Status anchor.Status
	Count  int
}

type entry struct {
	Path    string
	Name    string
	Href    string
	Count   int
	Worst   anchor.Status
	Current bool
	Search  string
}

type fileView struct {
	Path     string
	Lines    []line
	Notes    []note
	Detached []note
	Missing  bool
}

type line struct {
	Number int
	Text   string
	Marker anchor.Status
}

type note struct {
	ID      string
	Kind    string
	Status  anchor.Status
	Line    int
	Body    []string
	Created string
	Excerpt string
	Stale   bool
	Warning string
}

var statusOrder = []anchor.Status{
	anchor.StatusOK, anchor.StatusMoved, anchor.StatusAmbiguous, anchor.StatusDrifted, anchor.StatusOrphaned,
}

func build(repositorySnapshot *application.RepositorySnapshot, requested string, how links) (*view, error) {
	if len(repositorySnapshot.Files) == 0 {
		return &view{
			Empty: true, Stylesheet: how.stylesheet, Script: how.script,
			Home: how.home, LogoSVG: how.logoSVG, LogoPNG: how.logoPNG,
		}, nil
	}

	current := requested
	if current == "" {
		current = repositorySnapshot.Files[0].Path
	}

	built := &view{
		Current:    current,
		Home:       how.home,
		Stylesheet: how.stylesheet,
		Script:     how.script,
		LogoSVG:    how.logoSVG,
		LogoPNG:    how.logoPNG,
	}
	counts := map[anchor.Status]int{}
	listed := make([]entry, 0, len(repositorySnapshot.Files))

	for _, file := range repositorySnapshot.Files {
		worst := anchor.StatusOK
		var searchable strings.Builder
		searchable.WriteString(file.Path)
		for _, annotation := range file.Annotations {
			counts[annotation.Status]++
			built.Total++
			if statusSeverity[annotation.Status] > statusSeverity[worst] {
				worst = annotation.Status
			}
			searchable.WriteString("\n" + string(annotation.Record.Kind) + "\n" + annotation.Record.Body + "\n" + annotation.Record.Author.Name)
		}

		listed = append(listed, entry{
			Path:    file.Path,
			Name:    baseName(file.Path),
			Href:    how.file(file.Path),
			Count:   len(file.Annotations),
			Worst:   worst,
			Current: file.Path == current,
			Search:  strings.ToLower(searchable.String()),
		})

		if file.Path == current {
			built.File = buildFile(file)
		}
	}

	if built.File == nil {
		built.NotFound = true
	}
	built.Tally = tallyOf(counts)
	built.Tree, built.Loose = buildTree(listed, current)
	return built, nil
}

func baseName(file string) string {
	if cut := strings.LastIndex(file, "/"); cut >= 0 {
		return file[cut+1:]
	}
	return file
}

func tallyOf(counts map[anchor.Status]int) []tallyEntry {
	var tally []tallyEntry
	for _, status := range statusOrder {
		if counts[status] > 0 {
			tally = append(tally, tallyEntry{Status: status, Count: counts[status]})
		}
	}
	return tally
}

func buildFile(file application.FileSnapshot) *fileView {
	built := &fileView{Path: file.Path}
	if !file.Exists {
		built.Missing = true
		for _, annotation := range file.Annotations {
			built.Detached = append(built.Detached, describe(annotation))
		}
		return built
	}

	marked := map[int]anchor.Status{}
	for _, annotation := range file.Annotations {
		described := describe(annotation)
		if annotation.Line == 0 {
			built.Detached = append(built.Detached, described)
			continue
		}
		built.Notes = append(built.Notes, described)
		worst, seen := marked[annotation.Line]
		if !seen || statusSeverity[annotation.Status] > statusSeverity[worst] {
			marked[annotation.Line] = annotation.Status
		}
	}

	for i, text := range strings.Split(strings.TrimSuffix(string(file.Content), "\n"), "\n") {
		number := i + 1
		built.Lines = append(built.Lines, line{Number: number, Text: text, Marker: marked[number]})
	}
	return built
}

func describe(annotation application.AnnotationView) note {
	stale := annotation.Status.IsFailure()
	return note{
		ID:      annotation.Record.ID,
		Kind:    string(annotation.Record.Kind),
		Status:  annotation.Status,
		Line:    annotation.Line,
		Body:    store.Paragraphs(annotation.Record.Body),
		Created: annotation.Record.Created.Format("2006-01-02"),
		Excerpt: annotation.Record.Anchor.Excerpt,
		Stale:   stale,
		Warning: annotation.Warning,
	}
}
