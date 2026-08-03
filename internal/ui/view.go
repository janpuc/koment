package ui

import (
	"os"
	"path"
	"sort"
	"strings"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/store"
)

type view struct {
	Total      int
	Tally      []tallyEntry
	Groups     []group
	Current    string
	File       *fileView
	Empty      bool
	NotFound   bool
	Stylesheet string
	Repository string
	Snapshot   *snapshot
}

// snapshot is set only on a published page, and its presence is what makes the
// page admit to being one. The served UI re-reads the tree per request and so
// has nothing to confess (ADR 0026).
type snapshot struct {
	Commit     string
	CommitURL  string
	Banner     string
	BannerHref string
}

type links struct {
	file       func(target string) string
	stylesheet string
}

func servedLinks() links {
	return links{
		file:       func(target string) string { return filePrefix + target },
		stylesheet: "/assets/style.css",
	}
}

type tallyEntry struct {
	Status anchor.Status
	Count  int
}

type group struct {
	Dir   string
	Files []entry
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
	Detached []note
	Missing  bool
}

type line struct {
	Number int
	Text   string
	Notes  []note
}

type note struct {
	ID      string
	Kind    string
	Status  anchor.Status
	Body    []string
	Created string
	Excerpt string
	Stale   bool
	Warning string
}

// severity orders statuses so a file can advertise its worst one.
var severity = map[anchor.Status]int{
	anchor.StatusOK:       0,
	anchor.StatusMoved:    1,
	anchor.StatusDrifted:  2,
	anchor.StatusOrphaned: 3,
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
		return &view{Empty: true, Stylesheet: how.stylesheet}, nil
	}

	current := requested
	if current == "" {
		current = files[0]
	}

	built := &view{Current: current, Stylesheet: how.stylesheet}
	counts := map[anchor.Status]int{}
	byDirectory := map[string][]entry{}

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

		directory := path.Dir(file)
		byDirectory[directory] = append(byDirectory[directory], entry{
			Path:    file,
			Name:    path.Base(file),
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
	built.Groups = groupsOf(byDirectory)
	return built, nil
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

func groupsOf(byDirectory map[string][]entry) []group {
	directories := make([]string, 0, len(byDirectory))
	for directory := range byDirectory {
		directories = append(directories, directory)
	}
	sort.Strings(directories)

	groups := make([]group, 0, len(directories))
	for _, directory := range directories {
		files := byDirectory[directory]
		sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
		groups = append(groups, group{Dir: directory, Files: files})
	}
	return groups
}

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

	anchored := map[int][]note{}
	for _, resolution := range resolutions {
		described := describe(resolution)
		if resolution.Line == 0 {
			built.Detached = append(built.Detached, described)
			continue
		}
		anchored[resolution.Line] = append(anchored[resolution.Line], described)
	}

	for i, text := range strings.Split(strings.TrimSuffix(string(content), "\n"), "\n") {
		number := i + 1
		built.Lines = append(built.Lines, line{Number: number, Text: text, Notes: anchored[number]})
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
