// Package mcp serves koment annotations to agents over stdio or HTTP.
package mcp

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/metrics"
	"github.com/janpuc/koment/internal/store"
)

const (
	serverName    = "koment"
	serverVersion = "0.1.0"

	getDescription = "Annotations recorded against a source file: why it is written this way, " +
		"what bit someone here before, and which invariants must hold. Read this before editing " +
		"an unfamiliar file. Every annotation carries a resolution status; heed the warning field."

	searchDescription = "Full-text search across annotation bodies. Use it to find recorded rationale " +
		"by topic when you do not already know which file holds it."
)

func newServer(annotations *store.Store, recorder metrics.Recorder) *sdk.Server {
	server := sdk.NewServer(&sdk.Implementation{Name: serverName, Version: serverVersion}, nil)
	sdk.AddTool(server, &sdk.Tool{Name: "koment_get", Description: getDescription}, get(annotations, recorder))
	sdk.AddTool(server, &sdk.Tool{Name: "koment_search", Description: searchDescription}, search(annotations, recorder))
	return server
}

// measure records a tool call and, for each annotation handed over, its
// resolution status — so a rising drifted rate is visible as agents reading
// history as though it were current (ADR 0020).
func measure(recorder metrics.Recorder, tool string, started time.Time, served []Annotation, err error) {
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	recorder.ObserveMCPCall(tool, outcome, time.Since(started))
	for _, annotation := range served {
		recorder.ObserveServed(anchor.Status(annotation.Status))
	}
}

type GetInput struct {
	File string `json:"file" jsonschema:"path of the source file, relative to the repository root"`
}

type GetOutput struct {
	File        string       `json:"file"`
	Annotations []Annotation `json:"annotations"`
}

type SearchInput struct {
	Query string `json:"query" jsonschema:"text to look for in annotation bodies, matched case-insensitively"`
}

type SearchOutput struct {
	Query   string       `json:"query"`
	Matches []Annotation `json:"matches"`
}

type Annotation struct {
	File    string `json:"file"`
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Body    string `json:"body"`
	Scope   string `json:"scope"`
	Excerpt string `json:"excerpt,omitempty"`
	Line    int    `json:"line,omitempty"`
	Created string `json:"created"`
	Status  string `json:"status"`
	Warning string `json:"warning,omitempty"`
}

func get(annotations *store.Store, recorder metrics.Recorder) sdk.ToolHandlerFor[GetInput, GetOutput] {
	return func(_ context.Context, _ *sdk.CallToolRequest, input GetInput) (result *sdk.CallToolResult, out GetOutput, err error) {
		started := time.Now()
		defer func() { measure(recorder, "koment_get", started, out.Annotations, err) }()

		file, err := annotations.FromRoot(input.File)
		if err != nil {
			return nil, GetOutput{}, err
		}

		resolutions, err := anchor.ResolveStored(annotations, file)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, GetOutput{File: file, Annotations: []Annotation{}}, nil
		}
		if err != nil {
			return nil, GetOutput{}, err
		}
		return nil, GetOutput{File: file, Annotations: describeAll(file, resolutions)}, nil
	}
}

func search(annotations *store.Store, recorder metrics.Recorder) sdk.ToolHandlerFor[SearchInput, SearchOutput] {
	return func(_ context.Context, _ *sdk.CallToolRequest, input SearchInput) (result *sdk.CallToolResult, out SearchOutput, err error) {
		started := time.Now()
		defer func() { measure(recorder, "koment_search", started, out.Matches, err) }()

		query := strings.TrimSpace(input.Query)
		if query == "" {
			return nil, SearchOutput{}, errors.New("query must not be empty")
		}

		files, err := annotations.AnnotatedFiles()
		if err != nil {
			return nil, SearchOutput{}, err
		}

		matches := []Annotation{}
		for _, file := range files {
			resolutions, err := anchor.ResolveStored(annotations, file)
			if err != nil {
				return nil, SearchOutput{}, err
			}
			for _, resolution := range resolutions {
				if containsFold(resolution.Annotation.Body, query) {
					matches = append(matches, describe(file, resolution))
				}
			}
		}
		return nil, SearchOutput{Query: query, Matches: matches}, nil
	}
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func describeAll(file string, resolutions []anchor.Resolution) []Annotation {
	described := make([]Annotation, len(resolutions))
	for i, resolution := range resolutions {
		described[i] = describe(file, resolution)
	}
	return described
}

func describe(file string, resolution anchor.Resolution) Annotation {
	return Annotation{
		File:    file,
		ID:      resolution.Annotation.ID,
		Kind:    string(resolution.Annotation.Kind),
		Body:    resolution.Annotation.Body,
		Scope:   string(resolution.Annotation.Scope),
		Excerpt: resolution.Annotation.Excerpt,
		Line:    resolution.Line,
		Created: resolution.Annotation.Created.Format("2006-01-02"),
		Status:  string(resolution.Status),
		Warning: warningFor(resolution.Status),
	}
}

func warningFor(status anchor.Status) string {
	switch status {
	case anchor.StatusDrifted:
		return "STALE: the annotated code has changed and nobody revisited this note. " +
			"Treat it as history, not as a description of the code as it stands. Do not act on it without checking."
	case anchor.StatusOrphaned:
		return "STALE: the file this annotation described no longer exists. Treat it as history only."
	case anchor.StatusMoved:
		return "The annotated code is unchanged but has shifted position; line is where it is now."
	}
	return ""
}
