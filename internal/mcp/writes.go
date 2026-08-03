package mcp

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/repository"
	"github.com/janpuc/koment/internal/store"
)

const (
	addDescription         = "Create one attributed koment annotation. Use this for local rationale instead of adding an explanatory inline comment. The author is the connected MCP client and is recorded as an agent."
	reanchorDescription    = "Explicitly confirm a new file or excerpt for an existing annotation while preserving its stable id, author and creation date."
	convertDescription     = "Convert one complete Go comment group into an attributed koment annotation. The annotation is written before the comment is removed from source."
	acknowledgeDescription = "Retain one exceptional Go comment by creating an exact, attributable policy acknowledgement. acknowledge_inline_comment must be true; ordinary explanatory comments should use koment_convert_comment."
)

type AddInput struct {
	Repository string `json:"repository,omitempty" jsonschema:"repository id; required when several repositories are served"`
	File       string `json:"file" jsonschema:"source path relative to the repository root"`
	Excerpt    string `json:"excerpt,omitempty" jsonschema:"verbatim code excerpt; omit for a file-scoped annotation"`
	Kind       string `json:"kind" jsonschema:"one of why, gotcha, invariant, anti-pattern"`
	Body       string `json:"body" jsonschema:"the rationale to record"`
}

type ReanchorInput struct {
	Repository string `json:"repository,omitempty" jsonschema:"repository id; required when several repositories are served"`
	ID         string `json:"id" jsonschema:"stable annotation id"`
	File       string `json:"file,omitempty" jsonschema:"new repository-relative source path"`
	Excerpt    string `json:"excerpt,omitempty" jsonschema:"new verbatim code excerpt"`
}

type ConvertCommentInput struct {
	Repository string `json:"repository,omitempty" jsonschema:"repository id; required when several repositories are served"`
	File       string `json:"file" jsonschema:"source path relative to the repository root"`
	Comment    string `json:"comment" jsonschema:"complete verbatim Go comment group"`
	Kind       string `json:"kind,omitempty" jsonschema:"annotation kind; defaults to why"`
}

type AcknowledgeCommentInput struct {
	Repository   string `json:"repository,omitempty" jsonschema:"repository id; required when several repositories are served"`
	File         string `json:"file" jsonschema:"source path relative to the repository root"`
	Comment      string `json:"comment" jsonschema:"complete verbatim Go comment group to retain"`
	Body         string `json:"body" jsonschema:"why the normal koment procedure is insufficient"`
	Acknowledged bool   `json:"acknowledge_inline_comment" jsonschema:"must be true to waive the normal koment procedure"`
}

type MutationOutput struct {
	Repository string         `json:"repository"`
	Path       string         `json:"path"`
	Record     MutationRecord `json:"record"`
	Warnings   []string       `json:"warnings,omitempty"`
}

type MutationRecord struct {
	Version int               `json:"version"`
	ID      string            `json:"id"`
	File    string            `json:"file"`
	Kind    string            `json:"kind"`
	Body    string            `json:"body"`
	Created string            `json:"created"`
	Anchor  MutationAnchor    `json:"anchor"`
	Git     *AnnotationGit    `json:"git,omitempty"`
	Author  AnnotationAuthor  `json:"author"`
	Policy  *AnnotationPolicy `json:"policy,omitempty"`
}

type MutationAnchor struct {
	Scope        string `json:"scope"`
	Excerpt      string `json:"excerpt,omitempty"`
	Before       string `json:"before,omitempty"`
	After        string `json:"after,omitempty"`
	LastSeenLine int    `json:"last_seen_line,omitempty"`
}

func addWriteTools(server *sdk.Server, repositories *repository.Set) {
	sdk.AddTool(server, &sdk.Tool{Name: "koment_add", Description: addDescription}, add(repositories))
	sdk.AddTool(server, &sdk.Tool{Name: "koment_reanchor", Description: reanchorDescription}, reanchor(repositories))
	sdk.AddTool(server, &sdk.Tool{Name: "koment_convert_comment", Description: convertDescription}, convertComment(repositories))
	sdk.AddTool(server, &sdk.Tool{Name: "koment_acknowledge_comment", Description: acknowledgeDescription}, acknowledgeComment(repositories))
}

func add(repositories *repository.Set) sdk.ToolHandlerFor[AddInput, MutationOutput] {
	return func(_ context.Context, request *sdk.CallToolRequest, input AddInput) (*sdk.CallToolResult, MutationOutput, error) {
		entry, err := forWrite(repositories, input.Repository)
		if err != nil {
			return nil, MutationOutput{}, err
		}
		kind, err := store.ParseKind(input.Kind)
		if err != nil {
			return nil, MutationOutput{}, err
		}
		mutation, err := application.NewService(entry).Add(application.AddInput{
			File: input.File, Excerpt: input.Excerpt, Kind: kind, Body: input.Body, Author: agentAuthor(request),
		})
		if err != nil {
			return nil, MutationOutput{}, err
		}
		return nil, mutationOutput(entry.ID, mutation), nil
	}
}

func reanchor(repositories *repository.Set) sdk.ToolHandlerFor[ReanchorInput, MutationOutput] {
	return func(_ context.Context, _ *sdk.CallToolRequest, input ReanchorInput) (*sdk.CallToolResult, MutationOutput, error) {
		if strings.TrimSpace(input.ID) == "" {
			return nil, MutationOutput{}, fmt.Errorf("id must not be empty")
		}
		if input.File == "" && input.Excerpt == "" {
			return nil, MutationOutput{}, fmt.Errorf("reanchor needs file, excerpt, or both")
		}
		entry, err := forWrite(repositories, input.Repository)
		if err != nil {
			return nil, MutationOutput{}, err
		}
		mutation, err := application.NewService(entry).Reanchor(application.ReanchorInput{
			ID: input.ID, File: input.File, Excerpt: input.Excerpt,
		})
		if err != nil {
			return nil, MutationOutput{}, err
		}
		return nil, mutationOutput(entry.ID, mutation), nil
	}
}

func convertComment(repositories *repository.Set) sdk.ToolHandlerFor[ConvertCommentInput, MutationOutput] {
	return func(_ context.Context, request *sdk.CallToolRequest, input ConvertCommentInput) (*sdk.CallToolResult, MutationOutput, error) {
		entry, err := forWrite(repositories, input.Repository)
		if err != nil {
			return nil, MutationOutput{}, err
		}
		kind := store.KindWhy
		if input.Kind != "" {
			if kind, err = store.ParseKind(input.Kind); err != nil {
				return nil, MutationOutput{}, err
			}
		}
		mutation, err := application.NewService(entry).ConvertComment(application.ConvertCommentInput{
			File: input.File, Comment: input.Comment, Kind: kind, Author: agentAuthor(request),
		})
		if err != nil {
			return nil, MutationOutput{}, err
		}
		return nil, mutationOutput(entry.ID, mutation), nil
	}
}

func acknowledgeComment(repositories *repository.Set) sdk.ToolHandlerFor[AcknowledgeCommentInput, MutationOutput] {
	return func(_ context.Context, request *sdk.CallToolRequest, input AcknowledgeCommentInput) (*sdk.CallToolResult, MutationOutput, error) {
		entry, err := forWrite(repositories, input.Repository)
		if err != nil {
			return nil, MutationOutput{}, err
		}
		mutation, err := application.NewService(entry).AcknowledgeComment(application.AcknowledgeCommentInput{
			File: input.File, Comment: input.Comment, Body: input.Body,
			Author: agentAuthor(request), Acknowledged: input.Acknowledged,
		})
		if err != nil {
			return nil, MutationOutput{}, err
		}
		return nil, mutationOutput(entry.ID, mutation), nil
	}
}

func forWrite(repositories *repository.Set, named string) (repository.Repository, error) {
	if named != "" {
		entry, found := repositories.Resolve(named)
		if !found {
			return repository.Repository{}, fmt.Errorf("no repository %q; served: %s", named, strings.Join(repositories.IDs(), ", "))
		}
		return entry, nil
	}
	if entry, only := repositories.Only(); only {
		return entry, nil
	}
	return repository.Repository{}, fmt.Errorf("write requires repository; served: %s", strings.Join(repositories.IDs(), ", "))
}

func agentAuthor(request *sdk.CallToolRequest) store.Author {
	name := "MCP agent"
	if client := request.ClientInfo(); client != nil && strings.TrimSpace(client.Name) != "" {
		name = client.Name
	}
	return store.Author{Name: name, Kind: store.AuthorAgent, Source: store.FromSession}
}

func mutationOutput(repositoryID string, mutation application.Mutation) MutationOutput {
	record := mutation.Record
	output := MutationOutput{
		Repository: repositoryID, Path: mutation.Path, Warnings: mutation.Warnings,
		Record: MutationRecord{
			Version: record.Version, ID: record.ID, File: record.File, Kind: string(record.Kind),
			Body: record.Body, Created: record.Created.Format("2006-01-02"),
			Anchor: MutationAnchor{
				Scope: string(record.Anchor.Scope), Excerpt: record.Anchor.Excerpt,
				Before: record.Anchor.Before, After: record.Anchor.After, LastSeenLine: record.Anchor.LastSeenLine,
			},
			Author: AnnotationAuthor{
				Name: record.Author.Name, Email: record.Author.Email, Kind: string(record.Author.Kind),
				Source: string(record.Author.Source), Account: record.Author.Account, Verified: record.Author.Verified,
			},
		},
	}
	if record.Git != nil {
		output.Record.Git = &AnnotationGit{
			Commit: record.Git.Commit, Path: record.Git.Path, Line: record.Git.Line, EndLine: record.Git.EndLine,
		}
	}
	if record.Policy != nil {
		output.Record.Policy = &AnnotationPolicy{Exception: record.Policy.Exception, Acknowledged: record.Policy.Acknowledged}
	}
	return output
}
