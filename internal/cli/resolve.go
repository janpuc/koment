package cli

import (
	"fmt"
	"strings"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/store"
)

type fileResolutions struct {
	file        string
	resolutions []anchor.Resolution
}

func resolveEverything(annotations *store.Store, under []string) ([]fileResolutions, error) {
	files, err := annotations.AnnotatedFiles()
	if err != nil {
		return nil, err
	}

	prefixes, err := relativePrefixes(annotations, under)
	if err != nil {
		return nil, err
	}

	var resolved []fileResolutions
	for _, file := range files {
		if !covers(prefixes, file) {
			continue
		}
		resolutions, err := anchor.ResolveStored(annotations, file)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, fileResolutions{file: file, resolutions: resolutions})
	}
	return resolved, nil
}

func relativePrefixes(annotations *store.Store, paths []string) ([]string, error) {
	prefixes := make([]string, 0, len(paths))
	for _, path := range paths {
		relative, err := annotations.FromWorkingDirectory(path)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, relative)
	}
	return prefixes, nil
}

func covers(prefixes []string, file string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, prefix := range prefixes {
		if file == prefix || strings.HasPrefix(file, prefix+"/") {
			return true
		}
	}
	return false
}

var statusOrder = []anchor.Status{
	anchor.StatusOK,
	anchor.StatusMoved,
	anchor.StatusDrifted,
	anchor.StatusOrphaned,
}

type tally map[anchor.Status]int

func (t tally) add(status anchor.Status) { t[status]++ }

func (t tally) total() int {
	total := 0
	for _, count := range t {
		total += count
	}
	return total
}

func (t tally) failures() int {
	return t[anchor.StatusDrifted] + t[anchor.StatusOrphaned]
}

func (t tally) String() string {
	var parts []string
	for _, status := range statusOrder {
		if count := t[status]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, status))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func tallyOf(resolved []fileResolutions) tally {
	counted := tally{}
	for _, entry := range resolved {
		for _, resolution := range entry.resolutions {
			counted.add(resolution.Status)
		}
	}
	return counted
}
