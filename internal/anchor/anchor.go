// Package anchor decides where an annotation still applies, and says so when it
// no longer does.
package anchor

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/janpuc/koment/internal/store"
)

type Status string

const (
	StatusOK        Status = "ok"
	StatusMoved     Status = "moved"
	StatusAmbiguous Status = "ambiguous"
	StatusDrifted   Status = "drifted"
	StatusOrphaned  Status = "orphaned"
)

func (s Status) IsFailure() bool {
	return s == StatusAmbiguous || s == StatusDrifted || s == StatusOrphaned
}

type Resolution struct {
	Annotation  store.Annotation
	Status      Status
	Line        int
	Occurrences int
}

type occurrence struct {
	line      int
	startLine int
	endLine   int
	before    []string
	after     []string
}

func Resolve(annotation store.Annotation, content []byte) Resolution {
	if annotation.Anchor.Scope == store.ScopeFile {
		return Resolution{Annotation: annotation, Status: StatusOK}
	}

	found := findOccurrences(content, annotation.Anchor.Excerpt)
	if len(found) == 0 {
		return Resolution{Annotation: annotation, Status: StatusDrifted}
	}
	if len(found) == 1 {
		return resolved(annotation, found[0], 1)
	}

	contextual := filterByContext(found, annotation.Anchor)
	if len(contextual) != 1 {
		return Resolution{Annotation: annotation, Status: StatusAmbiguous, Occurrences: len(found)}
	}
	return resolved(annotation, contextual[0], len(found))
}

func resolved(annotation store.Annotation, found occurrence, count int) Resolution {
	status := StatusMoved
	if found.line == annotation.Anchor.LastSeenLine {
		status = StatusOK
	}
	return Resolution{Annotation: annotation, Status: status, Line: found.line, Occurrences: count}
}

func ResolveOrphaned(annotation store.Annotation) Resolution {
	return Resolution{Annotation: annotation, Status: StatusOrphaned}
}

func ResolveStored(annotations *store.Store, file string) ([]Resolution, error) {
	records, err := annotations.ForFile(file)
	if err != nil {
		return nil, err
	}
	content, err := annotations.ReadSource(file)
	if errors.Is(err, fs.ErrNotExist) {
		return resolveAll(records, ResolveOrphaned), nil
	}
	if err != nil {
		return nil, err
	}
	return resolveAll(records, func(annotation store.Annotation) Resolution {
		return Resolve(annotation, content)
	}), nil
}

func ResolveRecord(annotation store.Annotation, sourcePath string) (Resolution, error) {
	resolved, err := ResolveRecords([]store.Annotation{annotation}, sourcePath)
	if err != nil {
		return Resolution{}, err
	}
	return resolved[0], nil
}

func ResolveRecords(annotations []store.Annotation, sourcePath string) ([]Resolution, error) {
	content, err := os.ReadFile(sourcePath)
	if errors.Is(err, fs.ErrNotExist) {
		return resolveAll(annotations, ResolveOrphaned), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", sourcePath, err)
	}
	return resolveAll(annotations, func(annotation store.Annotation) Resolution {
		return Resolve(annotation, content)
	}), nil
}

func resolveAll(annotations []store.Annotation, resolve func(store.Annotation) Resolution) []Resolution {
	resolutions := make([]Resolution, len(annotations))
	for index, annotation := range annotations {
		resolutions[index] = resolve(annotation)
	}
	return resolutions
}

func Capture(content []byte, excerpt string) (store.Anchor, error) {
	found := findOccurrences(content, excerpt)
	switch len(found) {
	case 0:
		return store.Anchor{}, fmt.Errorf("excerpt does not occur in the source")
	case 1:
		return anchorFrom(found[0], excerpt), nil
	default:
		return store.Anchor{}, fmt.Errorf("excerpt occurs %d times; provide a more specific excerpt", len(found))
	}
}

func anchorFrom(found occurrence, excerpt string) store.Anchor {
	return store.Anchor{
		Scope:        store.ScopeExcerpt,
		Excerpt:      excerpt,
		Before:       strings.Join(last(found.before, 3), "\n"),
		After:        strings.Join(first(found.after, 3), "\n"),
		LastSeenLine: found.line,
	}
}

func filterByContext(found []occurrence, anchor store.Anchor) []occurrence {
	wantBefore := contextLines(anchor.Before)
	wantAfter := contextLines(anchor.After)
	filtered := make([]occurrence, 0, len(found))
	for _, candidate := range found {
		if equalStrings(last(candidate.before, len(wantBefore)), wantBefore) &&
			equalStrings(first(candidate.after, len(wantAfter)), wantAfter) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func contextLines(context string) []string {
	if context == "" {
		return nil
	}
	return strings.Split(context, "\n")
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func first(lines []string, count int) []string {
	if count > len(lines) {
		count = len(lines)
	}
	return lines[:count]
}

func last(lines []string, count int) []string {
	if count > len(lines) {
		count = len(lines)
	}
	return lines[len(lines)-count:]
}

func findOccurrences(content []byte, excerpt string) []occurrence {
	needle := []byte(excerpt)
	if len(needle) == 0 {
		return nil
	}
	lines, starts := splitLines(content)

	var found []occurrence
	for searched := 0; searched <= len(content)-len(needle); {
		index := bytes.Index(content[searched:], needle)
		if index < 0 {
			break
		}
		start := searched + index
		end := start + len(needle) - 1
		startLine := lineAt(starts, start)
		endLine := lineAt(starts, end)
		found = append(found, occurrence{
			line:      startLine + 1,
			startLine: startLine,
			endLine:   endLine,
			before:    lines[:startLine],
			after:     lines[endLine+1:],
		})
		searched = start + 1
	}
	return found
}

func splitLines(content []byte) ([]string, []int) {
	if len(content) == 0 {
		return []string{""}, []int{0}
	}
	starts := []int{0}
	for index, character := range content {
		if character == '\n' && index+1 < len(content) {
			starts = append(starts, index+1)
		}
	}
	lines := make([]string, len(starts))
	for index, start := range starts {
		end := len(content)
		if index+1 < len(starts) {
			end = starts[index+1] - 1
		} else if end > start && content[end-1] == '\n' {
			end--
		}
		if end > start && content[end-1] == '\r' {
			end--
		}
		lines[index] = string(content[start:end])
	}
	return lines, starts
}

func lineAt(starts []int, offset int) int {
	low, high := 0, len(starts)
	for low < high {
		middle := low + (high-low)/2
		if starts[middle] <= offset {
			low = middle + 1
		} else {
			high = middle
		}
	}
	return low - 1
}

func ExcerptLines(content []byte, excerpt string) []int {
	found := findOccurrences(content, excerpt)
	if len(found) == 0 {
		return nil
	}
	lines := make([]int, len(found))
	for index, occurrence := range found {
		lines[index] = occurrence.line
	}
	return lines
}
