// Package mcp serves koment annotations to agents over stdio or HTTP.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/janpuc/koment/internal/agentpolicy"
	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/application"
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

func newServer(repositories *repository.Set, recorder metrics.Recorder, writes bool) *sdk.Server {
	instructions := agentpolicy.Contract()
	if !writes {
		instructions += "\n\nThis server is read-only. Restart it with `koment mcp --write` over stdio when mutations are required."
	}
	server := sdk.NewServer(&sdk.Implementation{Name: serverName, Version: serverVersion}, &sdk.ServerOptions{Instructions: instructions})
	sdk.AddTool(server, &sdk.Tool{Name: "koment_get", Description: getDescription}, get(repositories, recorder))
	sdk.AddTool(server, &sdk.Tool{Name: "koment_search", Description: searchDescription}, search(repositories, recorder))
	sdk.AddTool(server, &sdk.Tool{Name: "koment_repositories", Description: repositoriesDescription}, list(repositories))
	if writes {
		addWriteTools(server, repositories)
	}
	return server
}

func repositoryForGet(repositories *repository.Set, named, file string) (repository.Repository, error) {
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
			snapshot, err := application.BuildSnapshot(entry)
			if err != nil {
				return nil, RepositoriesOutput{}, err
			}
			counts := map[string]int{}
			for status, count := range snapshot.Counts() {
				counts[string(status)] = count
			}
			summaries = append(summaries, RepositorySummary{
				ID: entry.ID, Name: entry.Display(),
				DefaultBranch: entry.DefaultBranch, CloneURL: entry.CloneURL,
				Files: len(snapshot.Files), Annotations: counts,
			})
		}
		return nil, RepositoriesOutput{Repositories: summaries}, nil
	}
}

func recordMCPCall(recorder metrics.Recorder, tool string, started time.Time, served []Annotation, err error) {
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
	Commit        string         `json:"commit,omitempty"`
	Files         int            `json:"files"`
	Annotations   map[string]int `json:"annotations"`
}

type GetOutput struct {
	Repository  string       `json:"repository"`
	Commit      string       `json:"commit,omitempty"`
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
	Repository  string            `json:"repository"`
	Commit      string            `json:"commit,omitempty"`
	File        string            `json:"file"`
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Body        string            `json:"body"`
	Scope       string            `json:"scope"`
	Excerpt     string            `json:"excerpt,omitempty"`
	Line        int               `json:"line,omitempty"`
	Occurrences int               `json:"occurrences"`
	Created     string            `json:"created"`
	Status      string            `json:"status"`
	Warning     string            `json:"warning,omitempty"`
	Author      AnnotationAuthor  `json:"author"`
	Git         *AnnotationGit    `json:"git,omitempty"`
	Policy      *AnnotationPolicy `json:"policy,omitempty"`
}

type AnnotationAuthor struct {
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Kind     string `json:"kind"`
	Source   string `json:"source"`
	Account  string `json:"account,omitempty"`
	Verified string `json:"verified,omitempty"`
}

type AnnotationGit struct {
	Commit  string `json:"commit"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	EndLine int    `json:"end_line,omitempty"`
}

type AnnotationPolicy struct {
	Exception    string `json:"exception"`
	Acknowledged bool   `json:"acknowledged"`
}

func get(repositories *repository.Set, recorder metrics.Recorder) sdk.ToolHandlerFor[GetInput, GetOutput] {
	return func(_ context.Context, _ *sdk.CallToolRequest, input GetInput) (result *sdk.CallToolResult, out GetOutput, err error) {
		started := time.Now()
		defer func() { recordMCPCall(recorder, "koment_get", started, out.Annotations, err) }()

		chosen, err := repositoryForGet(repositories, input.Repository, input.File)
		if err != nil {
			return nil, GetOutput{}, err
		}
		annotations := chosen.Store()
		file, err := annotations.FromRoot(input.File)
		if err != nil {
			return nil, GetOutput{}, err
		}
		snapshot, err := application.BuildSnapshot(chosen)
		if err != nil {
			return nil, GetOutput{}, err
		}
		fileSnapshot, found := snapshot.File(file)
		views := []application.AnnotationView{}
		if found {
			views = fileSnapshot.Annotations
		}
		return nil, GetOutput{
			File: file, Repository: chosen.ID,
			Annotations: describeAll(chosen.ID, views),
		}, nil
	}
}

func search(repositories *repository.Set, recorder metrics.Recorder) sdk.ToolHandlerFor[SearchInput, SearchOutput] {
	return func(_ context.Context, _ *sdk.CallToolRequest, input SearchInput) (result *sdk.CallToolResult, out SearchOutput, err error) {
		started := time.Now()
		defer func() { recordMCPCall(recorder, "koment_search", started, out.Matches, err) }()

		query := strings.TrimSpace(input.Query)
		if query == "" {
			return nil, SearchOutput{}, errors.New("query must not be empty")
		}

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
			snapshot, err := application.BuildSnapshot(entry)
			if err != nil {
				return nil, SearchOutput{}, err
			}
			for _, view := range snapshot.Search(query) {
				matches = append(matches, describe(entry.ID, view))
			}
		}
		return nil, SearchOutput{Query: query, Matches: matches}, nil
	}
}

func describeAll(repositoryID string, views []application.AnnotationView) []Annotation {
	described := make([]Annotation, len(views))
	for index, view := range views {
		described[index] = describe(repositoryID, view)
	}
	return described
}

func describe(repositoryID string, view application.AnnotationView) Annotation {
	record := view.Record
	described := Annotation{
		Repository:  repositoryID,
		File:        record.Spec.Target.File,
		ID:          record.Metadata.ID,
		Kind:        string(record.Spec.Type),
		Body:        record.Spec.Body,
		Scope:       string(record.Spec.Anchor.Scope),
		Excerpt:     record.Spec.Anchor.Excerpt,
		Line:        view.Line,
		Occurrences: view.Occurrences,
		Created:     record.Metadata.Created.Format("2006-01-02"),
		Status:      string(view.Status),
		Warning:     view.Warning,
		Author: AnnotationAuthor{
			Name: record.Spec.Author.Name, Email: record.Spec.Author.Email, Kind: string(record.Spec.Author.Kind),
			Source: string(record.Spec.Author.Source), Account: record.Spec.Author.Account, Verified: record.Spec.Author.Verified,
		},
	}
	if record.Spec.Git != nil {
		described.Git = &AnnotationGit{
			Commit: record.Spec.Git.Commit, Path: record.Spec.Git.Path, Line: record.Spec.Git.Line, EndLine: record.Spec.Git.EndLine,
		}
	}
	if record.Spec.Policy != nil {
		described.Policy = &AnnotationPolicy{Exception: record.Spec.Policy.Exception, Acknowledged: record.Spec.Policy.Acknowledged}
	}
	return described
}
