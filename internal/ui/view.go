package ui

import (
	"os"
	"strings"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/store"
)

type view struct {
	Total        int
	Tally        []tallyEntry
	Tree         []node
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
}

// snapshot is set only on a published page.
type snapshot struct {
	Commit     string
	CommitURL  string
	Banner     string
	BannerHref string
}

// repositoryLink is one entry in the switcher.
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
		file:       func(target string) string { return base + "f/" + target },
		home:       base,
		stylesheet: "/assets/style.css",
		script:     "/assets/koment.js",
		logoSVG:    "/assets/koment-logo.svg",
		logoPNG:    "/assets/koment-logo.png",
	}
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
	anchor.StatusOK, anchor.StatusMoved, anchor.StatusDrifted, anchor.StatusOrphaned,
}

func build(annotations *store.Store, requested string, how links) (*view, error) {
	files, err := annotations.AnnotatedFiles()
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return &view{
			Empty: true, Stylesheet: how.stylesheet, Script: how.script,
			Home: how.home, LogoSVG: how.logoSVG, LogoPNG: how.logoPNG,
		}, nil
	}

	current := requested
	if current == "" {
		current = files[0]
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
	listed := make([]entry, 0, len(files))

	for _, file := range files {
		resolutions, err := anchor.ResolveStored(annotations, file)
		if err != nil {
			return nil, err
		}

		worst := anchor.StatusOK
		for _, resolution := range resolutions {
			counts[resolution.Status]++
			built.Total++
			if severity[resolution.Status] > severity[worst] {
				worst = resolution.Status
			}
		}

		listed = append(listed, entry{
			Path:    file,
			Name:    baseName(file),
			Href:    how.file(file),
			Count:   len(resolutions),
			Worst:   worst,
			Current: file == current,
		})

		if file == current {
			if built.File, err = buildFile(annotations, file, resolutions); err != nil {
				return nil, err
			}
		}
	}

	if built.File == nil {
		built.NotFound = true
	}
	built.Tally = tallyOf(counts)
	built.Tree, built.Loose = treeOf(listed, current)
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

// buildFile renders the source as uniform lines and the notes separately, each
// carrying the line it belongs to.
func buildFile(annotations *store.Store, file string, resolutions []anchor.Resolution) (*fileView, error) {
	built := &fileView{Path: file}

	sourcePath, err := annotations.SourcePath(file)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(sourcePath)
	if os.IsNotExist(err) {
		built.Missing = true
		for _, resolution := range resolutions {
			built.Detached = append(built.Detached, describe(resolution))
		}
		return built, nil
	}
	if err != nil {
		return nil, err
	}

	marked := map[int]anchor.Status{}
	for _, resolution := range resolutions {
		described := describe(resolution)
		if resolution.Line == 0 {
			built.Detached = append(built.Detached, described)
			continue
		}
		built.Notes = append(built.Notes, described)
		worst, seen := marked[resolution.Line]
		if !seen || severity[resolution.Status] > severity[worst] {
			marked[resolution.Line] = resolution.Status
		}
	}

	for i, text := range strings.Split(strings.TrimSuffix(string(content), "\n"), "\n") {
		number := i + 1
		built.Lines = append(built.Lines, line{Number: number, Text: text, Marker: marked[number]})
	}
	return built, nil
}

// describe renders a drifted annotation as history: its excerpt is shown as
// what used to be there, never laid over the current source (ADR 0005).
func describe(resolution anchor.Resolution) note {
	stale := resolution.Status.IsFailure()
	return note{
		ID:      resolution.Annotation.ID,
		Kind:    string(resolution.Annotation.Kind),
		Status:  resolution.Status,
		Line:    resolution.Line,
		Body:    store.Paragraphs(resolution.Annotation.Body),
		Created: resolution.Annotation.Created.Format("2006-01-02"),
		Excerpt: resolution.Annotation.Excerpt,
		Stale:   stale,
		Warning: warningFor(resolution.Status),
	}
}

func warningFor(status anchor.Status) string {
	switch status {
	case anchor.StatusDrifted:
		return "The annotated code changed and nobody revisited this note. Treat it as history, not as a description of the code as it stands."
	case anchor.StatusOrphaned:
		return "The file this annotation described no longer exists. Treat it as history only."
	}
	return ""
}
