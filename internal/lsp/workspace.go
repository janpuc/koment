package lsp

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/commentpolicy"
	"github.com/janpuc/koment/internal/policy"
	"github.com/janpuc/koment/internal/repository"
	"github.com/janpuc/koment/internal/store"
)

type workspaceFile struct {
	root     string
	relative string
	content  []byte
	service  *application.Service
	store    *store.Store
}

func loadWorkspaceFile(uri string, content []byte) (workspaceFile, error) {
	absolute, err := pathFromURI(uri)
	if err != nil {
		return workspaceFile{}, err
	}
	root, err := store.FindRoot(filepath.Dir(absolute))
	if err != nil {
		return workspaceFile{}, err
	}
	annotations := store.Open(root)
	relative, err := annotations.FromWorkingDirectory(absolute)
	if err != nil {
		return workspaceFile{}, err
	}
	if content == nil {
		content, err = annotations.ReadSource(relative)
		if err != nil {
			return workspaceFile{}, fmt.Errorf("reading %s: %w", relative, err)
		}
	}
	entry := repository.Repository{ID: filepath.Base(root), Name: filepath.Base(root), Root: root}
	return workspaceFile{
		root: root, relative: relative, content: content,
		service: application.NewService(entry), store: annotations,
	}, nil
}

func pathFromURI(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" {
		return "", fmt.Errorf("URI %q is not a local file", uri)
	}
	value, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil {
		return "", fmt.Errorf("decoding URI %q: %w", uri, err)
	}
	if runtime.GOOS == "windows" && len(value) >= 3 && value[0] == '/' && value[2] == ':' {
		value = value[1:]
	}
	return filepath.Clean(filepath.FromSlash(value)), nil
}

func annotationViews(file workspaceFile) ([]application.AnnotationView, error) {
	records, err := file.store.ForFile(file.relative)
	if err != nil {
		return nil, err
	}
	snapshot, err := application.AssembleSnapshot(application.SnapshotInput{
		Repository: application.RepositoryIdentity{ID: filepath.Base(file.root), Name: filepath.Base(file.root)},
		Records:    records, Sources: map[string][]byte{file.relative: file.content},
	})
	if err != nil {
		return nil, err
	}
	resolved, exists := snapshot.File(file.relative)
	if !exists {
		return nil, nil
	}
	return resolved.Annotations, nil
}

func annotationItems(file workspaceFile) ([]annotationItem, error) {
	views, err := annotationViews(file)
	if err != nil {
		return nil, err
	}
	items := make([]annotationItem, 0, len(views))
	for _, view := range views {
		line := max(1, view.Line)
		if view.Record.Anchor.Scope == store.ScopeFile {
			line = 1
		}
		annotationRange := rangeValue{
			Start: position{Line: line - 1},
			End:   position{Line: line - 1, Character: lineUTF16Length(file.content, line-1)},
		}
		items = append(items, annotationItem{
			ID: view.Record.ID, Kind: string(view.Record.Kind), Body: view.Record.Body,
			Status: string(view.Status), Line: line, Warning: view.Warning, Range: annotationRange,
		})
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].Line != items[right].Line {
			return items[left].Line < items[right].Line
		}
		return items[left].ID < items[right].ID
	})
	return items, nil
}

func documentDiagnostics(file workspaceFile) ([]diagnostic, error) {
	items, err := annotationItems(file)
	if err != nil {
		return nil, err
	}
	diagnostics := []diagnostic{}
	for _, item := range items {
		switch anchor.Status(item.Status) {
		case anchor.StatusAmbiguous, anchor.StatusDrifted, anchor.StatusOrphaned:
			diagnostics = append(diagnostics, diagnostic{
				Range: item.Range, Severity: 1, Code: "koment." + item.Status,
				Source: "koment", Message: item.Warning, Data: map[string]string{"id": item.ID},
			})
		case anchor.StatusMoved:
			diagnostics = append(diagnostics, diagnostic{
				Range: item.Range, Severity: 3, Code: "koment.moved", Source: "koment",
				Message: "Annotation resolves here but its recorded line moved; reanchor it to refresh provenance.",
				Data:    map[string]string{"id": item.ID},
			})
		}
	}
	if filepath.Ext(file.relative) != ".go" {
		return diagnostics, nil
	}
	configured, err := policy.Load(file.root)
	if errors.Is(err, fs.ErrNotExist) {
		return diagnostics, nil
	}
	if err != nil {
		return nil, err
	}
	records, err := file.store.ForFile(file.relative)
	if err != nil {
		return nil, err
	}
	violations, err := commentpolicy.CheckContent(file.relative, file.content, configured, records)
	if err != nil {
		return nil, err
	}
	for _, violation := range violations {
		diagnostics = append(diagnostics, diagnostic{
			Range:    rangeFromOffsets(file.content, violation.Comment.Start, violation.Comment.End),
			Severity: 2, Code: "koment.comment", Source: "koment",
			Message: violation.Reason,
			Data: map[string]any{
				"comment": violation.Comment.Raw, "file": file.relative,
				"autoPrompt": commentpolicy.IsCommentIntent(violation.Comment),
			},
		})
	}
	return diagnostics, nil
}

func rangeFromOffsets(content []byte, start, end int) rangeValue {
	return rangeValue{Start: positionAt(content, start), End: positionAt(content, end)}
}

func positionAt(content []byte, offset int) position {
	offset = min(max(offset, 0), len(content))
	lineStart := 0
	line := 0
	for index, character := range content[:offset] {
		if character == '\n' {
			line++
			lineStart = index + 1
		}
	}
	units := 0
	for remaining := content[lineStart:offset]; len(remaining) > 0; {
		character, size := utf8.DecodeRune(remaining)
		units += len(utf16.Encode([]rune{character}))
		remaining = remaining[size:]
	}
	return position{Line: line, Character: units}
}

func lineUTF16Length(content []byte, wanted int) int {
	start := 0
	line := 0
	for index, character := range content {
		if line == wanted && character == '\n' {
			return positionAt(content, index).Character
		}
		if character == '\n' {
			line++
			start = index + 1
		}
	}
	if line == wanted {
		return positionAt(content, len(content)).Character
	}
	_ = start
	return 0
}

func markdown(item annotationItem) string {
	var text strings.Builder
	fmt.Fprintf(&text, "**%s** · `%s` · `%s`\n\n%s", item.Kind, item.Status, item.ID, item.Body)
	if item.Warning != "" {
		fmt.Fprintf(&text, "\n\n> %s", item.Warning)
	}
	return text.String()
}
