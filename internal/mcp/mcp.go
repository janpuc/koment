// Package mcp serves koment annotations to agents over stdio or HTTP.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/metrics"
	"github.com/janpuc/koment/internal/repository"
)

const (
	serverName    = "koment"
	serverVersion = "0.1.0"

	getDescription = "Annotations recorded against a source file: why it is written this way, " +
		"what bit someone here before, and which invariants must hold. Read this before editing " +
		"an unfamiliar file. Every annotation carries a resolution status; heed the warning field. " +
		"Pass repository when more than one is served - call koment_repositories to see them. " +
		"Omitting it resolves only if exactly one repository has that path."

	searchDescription = "Full-text search across annotation bodies. Use it to find recorded rationale " +
		"by topic when you do not already know which file holds it. Omitting repository searches " +
		"every repository; each match names the one it came from."

	repositoriesDescription = "The repositories this koment serves, with their annotation counts. " +
		"Call this first when you do not know which repository a file belongs to."
)

func newServer(repositories *repository.Set, recorder metrics.Recorder) *sdk.Server {
	server := sdk.NewServer(&sdk.Implementation{Name: serverName, Version: serverVersion}, nil)
	sdk.AddTool(server, &sdk.Tool{Name: "koment_get", Description: getDescription}, get(repositories, recorder))
	sdk.AddTool(server, &sdk.Tool{Name: "koment_search", Description: searchDescription}, search(repositories, recorder))
	sdk.AddTool(server, &sdk.Tool{Name: "koment_repositories", Description: repositoriesDescription}, list(repositories))
	return server
}

// forGet picks the repository a path belongs to. With several candidates it
// refuses and names them rather than guessing, because answering an ambiguous
// question silently is the failure koment exists to prevent (ADR 0025).
func forGet(repositories *repository.Set, named, file string) (repository.Repository, error) {
	if named != "" {
		chosen, found := repositories.Resolve(named)
		if !found {
			return repository.Repository{}, fmt.Errorf("no repository %q; served: %s",
				named, strings.Join(repositories.IDs(), ", "))
		}
		return chosen, nil
	}
	if only, single := repositories.Only(); single {
		return only, nil
	}

	var candidates []repository.Repository
	for _, candidate := range repositories.All() {
		annotations := candidate.Store()
		candidateFile, err := annotations.FromRoot(file)
		if err != nil {
			continue
		}
		found, err := annotations.ForFile(candidateFile)
		if err != nil {
			return repository.Repository{}, err
		}
		if len(found) > 0 {
			candidates = append(candidates, candidate)
		}
	}

	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		return repository.Repository{}, fmt.Errorf("no repository has annotations for %s; served: %s",
			file, strings.Join(repositories.IDs(), ", "))
	default:
		names := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			names = append(names, candidate.ID)
		}
		return repository.Repository{}, fmt.Errorf(
			"%s is annotated in more than one repository (%s); pass repository to choose",
			file, strings.Join(names, ", "))
	}
}

func list(repositories *repository.Set) sdk.ToolHandlerFor[RepositoriesInput, RepositoriesOutput] {
	return func(_ context.Context, _ *sdk.CallToolRequest, _ RepositoriesInput) (*sdk.CallToolResult, RepositoriesOutput, error) {
		summaries := make([]RepositorySummary, 0, repositories.Len())
		for _, entry := range repositories.All() {
			counts := map[string]int{}
			files, err := entry.Store().AnnotatedFiles()
			if err != nil {
				return nil, RepositoriesOutput{}, err
			}
			for _, file := range files {
				resolutions, err := anchor.ResolveStored(entry.Store(), file)
				if err != nil {
					return nil, RepositoriesOutput{}, err
				}
				for _, resolution := range resolutions {
					counts[string(resolution.Status)]++
				}
			}
			summaries = append(summaries, RepositorySummary{
				ID: entry.ID, Name: entry.Display(),
				DefaultBranch: entry.DefaultBranch, CloneURL: entry.CloneURL,
				Files: len(files), Annotations: counts,
			})
		}
		return nil, RepositoriesOutput{Repositories: summaries}, nil
	}
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
	File       string `json:"file" jsonschema:"path of the source file, relative to the repository root"`
	Repository string `json:"repository,omitempty" jsonschema:"which repository; needed only when several serve this path"`
}

type RepositoriesInput struct{}

type RepositoriesOutput struct {
	Repositories []RepositorySummary `json:"repositories"`
}

type RepositorySummary struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	DefaultBranch string         `json:"default_branch,omitempty"`
	CloneURL      string         `json:"clone_url,omitempty"`
	Files         int            `json:"files"`
	Annotations   map[string]int `json:"annotations"`
}

type GetOutput struct {
	Repository  string       `json:"repository"`
	File        string       `json:"file"`
	Annotations []Annotation `json:"annotations"`
}

type SearchInput struct {
	Query      string `json:"query" jsonschema:"text to look for in annotation bodies, matched case-insensitively"`
	Repository string `json:"repository,omitempty" jsonschema:"limit to one repository; omit to search all of them"`
}

type SearchOutput struct {
	Query   string       `json:"query"`
	Matches []Annotation `json:"matches"`
}

type Annotation struct {
	Repository string `json:"repository"`
	File       string `json:"file"`
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Body       string `json:"body"`
	Scope      string `json:"scope"`
	Excerpt    string `json:"excerpt,omitempty"`
	Line       int    `json:"line,omitempty"`
	Created    string `json:"created"`
	Status     string `json:"status"`
	Warning    string `json:"warning,omitempty"`
}

func get(repositories *repository.Set, recorder metrics.Recorder) sdk.ToolHandlerFor[GetInput, GetOutput] {
	return func(_ context.Context, _ *sdk.CallToolRequest, input GetInput) (result *sdk.CallToolResult, out GetOutput, err error) {
		started := time.Now()
		defer func() { measure(recorder, "koment_get", started, out.Annotations, err) }()

		chosen, err := forGet(repositories, input.Repository, input.File)
		if err != nil {
			return nil, GetOutput{}, err
		}
		annotations := chosen.Store()

		file, err := annotations.FromRoot(input.File)
		if err != nil {
			return nil, GetOutput{}, err
		}

		resolutions, err := anchor.ResolveStored(annotations, file)
		if err != nil {
			return nil, GetOutput{}, err
		}
		return nil, GetOutput{
			File: file, Repository: chosen.ID,
			Annotations: describeAll(chosen.ID, file, resolutions),
		}, nil
	}
}

func search(repositories *repository.Set, recorder metrics.Recorder) sdk.ToolHandlerFor[SearchInput, SearchOutput] {
	return func(_ context.Context, _ *sdk.CallToolRequest, input SearchInput) (result *sdk.CallToolResult, out SearchOutput, err error) {
		started := time.Now()
		defer func() { measure(recorder, "koment_search", started, out.Matches, err) }()

		query := strings.TrimSpace(input.Query)
		if query == "" {
			return nil, SearchOutput{}, errors.New("query must not be empty")
		}

		// Unlike get, an omitted repository searches all of them: searching
		// broadly and naming where each hit came from is useful, where
		// refusing would not be (ADR 0025).
		searching := repositories.All()
		if input.Repository != "" {
			chosen, found := repositories.Resolve(input.Repository)
			if !found {
				return nil, SearchOutput{}, fmt.Errorf("no repository %q; served: %s",
					input.Repository, strings.Join(repositories.IDs(), ", "))
			}
			searching = []repository.Repository{chosen}
		}

		matches := []Annotation{}
		for _, entry := range searching {
			annotations := entry.Store()
			files, err := annotations.AnnotatedFiles()
			if err != nil {
				return nil, SearchOutput{}, err
			}
			for _, file := range files {
				resolutions, err := anchor.ResolveStored(annotations, file)
				if err != nil {
					return nil, SearchOutput{}, err
				}
				for _, resolution := range resolutions {
					if containsFold(resolution.Annotation.Body, query) {
						matches = append(matches, describe(entry.ID, file, resolution))
					}
				}
			}
		}
		return nil, SearchOutput{Query: query, Matches: matches}, nil
	}
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func describeAll(repositoryID, file string, resolutions []anchor.Resolution) []Annotation {
	described := make([]Annotation, len(resolutions))
	for i, resolution := range resolutions {
		described[i] = describe(repositoryID, file, resolution)
	}
	return described
}

func describe(repositoryID, file string, resolution anchor.Resolution) Annotation {
	return Annotation{
		Repository: repositoryID,
		File:       file,
		ID:         resolution.Annotation.ID,
		Kind:       string(resolution.Annotation.Kind),
		Body:       resolution.Annotation.Body,
		Scope:      string(resolution.Annotation.Anchor.Scope),
		Excerpt:    resolution.Annotation.Anchor.Excerpt,
		Line:       resolution.Line,
		Created:    resolution.Annotation.Created.Format("2006-01-02"),
		Status:     string(resolution.Status),
		Warning:    warningFor(resolution.Status),
	}
}

func warningFor(status anchor.Status) string {
	switch status {
	case anchor.StatusAmbiguous:
		return "STALE: the excerpt matches several places and its context does not identify one. " +
			"Treat it as history until someone explicitly reanchors it."
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
