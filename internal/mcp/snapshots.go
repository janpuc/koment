package mcp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/koment-dev/koment/internal/agentpolicy"
	"github.com/koment-dev/koment/internal/application"
	"github.com/koment-dev/koment/internal/metrics"
	"github.com/koment-dev/koment/internal/serving"
	"github.com/koment-dev/koment/internal/store"
)

type RepositoryAccess map[string]bool

func NewSnapshotServer(catalog *serving.Catalog, recorder metrics.Recorder, access RepositoryAccess) *sdk.Server {
	return newSnapshotServer(catalog, recorder, access, nil)
}

type snapshotWriter struct {
	author       store.Author
	materializer serving.Materializer
}

func NewWritableSnapshotServer(
	catalog *serving.Catalog, recorder metrics.Recorder, access RepositoryAccess,
	author store.Author, materializer serving.Materializer,
) *sdk.Server {
	return newSnapshotServer(catalog, recorder, access, &snapshotWriter{author: author, materializer: materializer})
}

func newSnapshotServer(catalog *serving.Catalog, recorder metrics.Recorder, access RepositoryAccess, writer *snapshotWriter) *sdk.Server {
	instructions := agentpolicy.Contract() + "\n\nThis remote session reads immutable, commit-stamped snapshots."
	if writer == nil {
		instructions += " Remote mutations are not authorized for this session."
	} else {
		instructions += " This served surface creates reviewed annotation pull requests. Reanchor and source-comment mutations require a local checkout and are not exposed here."
	}
	server := sdk.NewServer(
		&sdk.Implementation{Name: serverName, Version: serverVersion},
		&sdk.ServerOptions{Instructions: instructions},
	)
	sdk.AddTool(server, &sdk.Tool{Name: "koment_get", Description: getDescription}, snapshotGet(catalog, recorder, access))
	sdk.AddTool(server, &sdk.Tool{Name: "koment_search", Description: searchDescription}, snapshotSearch(catalog, recorder, access))
	sdk.AddTool(server, &sdk.Tool{Name: "koment_repositories", Description: repositoriesDescription}, snapshotList(catalog, access))
	if writer != nil {
		sdk.AddTool(server, &sdk.Tool{Name: "koment_add", Description: addDescription}, snapshotAdd(catalog, access, *writer))
	}
	return server
}

func snapshotAdd(catalog *serving.Catalog, access RepositoryAccess, writer snapshotWriter) sdk.ToolHandlerFor[AddInput, MutationOutput] {
	return func(ctx context.Context, _ *sdk.CallToolRequest, input AddInput) (*sdk.CallToolResult, MutationOutput, error) {
		if writer.materializer == nil {
			return nil, MutationOutput{}, errors.New("remote materializer is not configured")
		}
		kind, err := store.ParseType(input.Kind)
		if err != nil {
			return nil, MutationOutput{}, err
		}
		state, err := snapshotForWrite(catalog, access, input.Repository)
		if err != nil {
			return nil, MutationOutput{}, err
		}
		record, err := application.DraftAnnotation(state.Snapshot, application.AddInput{
			File: input.File, Excerpt: input.Excerpt, Kind: kind, Body: input.Body, Author: writer.author,
		})
		if err != nil {
			return nil, MutationOutput{}, err
		}
		materialized, err := writer.materializer.Materialize(ctx, state.Repository, state.Snapshot.Commit, record)
		if err != nil {
			return nil, MutationOutput{}, err
		}
		out := mutationOutput(state.Repository.Identity.ID, application.Mutation{
			Record: record, Path: store.DirName + "/annotations/" + record.Metadata.ID + ".yaml",
		})
		out.Review = &MutationReview{
			BaseCommit: state.Snapshot.Commit, Branch: materialized.Branch, Commit: materialized.Commit,
			PullRequest: materialized.PullRequest, URL: materialized.URL,
		}
		return nil, out, nil
	}
}

func snapshotForWrite(catalog *serving.Catalog, access RepositoryAccess, named string) (serving.State, error) {
	states := accessibleStates(catalog, access)
	if named != "" {
		state, found := resolveState(states, named)
		if !found {
			return serving.State{}, fmt.Errorf("no writable repository %q; allowed: %s", named, stateIDs(states))
		}
		if state.Snapshot == nil {
			return serving.State{}, fmt.Errorf("repository %s has no synchronized snapshot", state.Repository.Identity.ID)
		}
		return state, nil
	}
	if len(states) != 1 {
		return serving.State{}, fmt.Errorf("repository is required for a remote write; allowed: %s", stateIDs(states))
	}
	if states[0].Snapshot == nil {
		return serving.State{}, fmt.Errorf("repository %s has no synchronized snapshot", states[0].Repository.Identity.ID)
	}
	return states[0], nil
}

func snapshotList(catalog *serving.Catalog, access RepositoryAccess) sdk.ToolHandlerFor[RepositoriesInput, RepositoriesOutput] {
	return func(_ context.Context, _ *sdk.CallToolRequest, _ RepositoriesInput) (*sdk.CallToolResult, RepositoriesOutput, error) {
		states := accessibleStates(catalog, access)
		summaries := make([]RepositorySummary, 0, len(states))
		for _, state := range states {
			if state.Snapshot == nil {
				return nil, RepositoriesOutput{}, fmt.Errorf("repository %s has no synchronized snapshot", state.Repository.Identity.ID)
			}
			counts := map[string]int{}
			for status, count := range state.Snapshot.Counts() {
				counts[string(status)] = count
			}
			identity := state.Snapshot.Repository
			summaries = append(summaries, RepositorySummary{
				ID: identity.ID, Name: displayIdentity(identity), DefaultBranch: identity.DefaultBranch,
				CloneURL: identity.CloneURL, Commit: state.Snapshot.Commit,
				Files: len(state.Snapshot.Files), Annotations: counts,
			})
		}
		return nil, RepositoriesOutput{Repositories: summaries}, nil
	}
}

func snapshotGet(catalog *serving.Catalog, recorder metrics.Recorder, access RepositoryAccess) sdk.ToolHandlerFor[GetInput, GetOutput] {
	return func(_ context.Context, _ *sdk.CallToolRequest, input GetInput) (result *sdk.CallToolResult, out GetOutput, err error) {
		started := time.Now()
		defer func() { recordMCPCall(recorder, "koment_get", started, out.Annotations, err) }()

		file, err := validSnapshotPath(input.File)
		if err != nil {
			return nil, GetOutput{}, err
		}
		state, err := snapshotForGet(catalog, access, input.Repository, file)
		if err != nil {
			return nil, GetOutput{}, err
		}
		fileSnapshot, found := state.Snapshot.File(file)
		views := []application.AnnotationView{}
		if found {
			views = fileSnapshot.Annotations
		}
		annotations := describeAll(state.Repository.Identity.ID, views)
		for index := range annotations {
			annotations[index].Commit = state.Snapshot.Commit
		}
		return nil, GetOutput{
			Repository: state.Repository.Identity.ID, Commit: state.Snapshot.Commit,
			File: file, Annotations: annotations,
		}, nil
	}
}

func snapshotSearch(catalog *serving.Catalog, recorder metrics.Recorder, access RepositoryAccess) sdk.ToolHandlerFor[SearchInput, SearchOutput] {
	return func(_ context.Context, _ *sdk.CallToolRequest, input SearchInput) (result *sdk.CallToolResult, out SearchOutput, err error) {
		started := time.Now()
		defer func() { recordMCPCall(recorder, "koment_search", started, out.Matches, err) }()

		query := strings.TrimSpace(input.Query)
		if query == "" {
			return nil, SearchOutput{}, errors.New("query must not be empty")
		}
		states := accessibleStates(catalog, access)
		if input.Repository != "" {
			state, found := resolveState(states, input.Repository)
			if !found {
				return nil, SearchOutput{}, fmt.Errorf("no accessible repository %q; served: %s", input.Repository, stateIDs(states))
			}
			states = []serving.State{state}
		}
		matches := []Annotation{}
		for _, state := range states {
			if state.Snapshot == nil {
				return nil, SearchOutput{}, fmt.Errorf("repository %s has no synchronized snapshot", state.Repository.Identity.ID)
			}
			for _, view := range state.Snapshot.Search(query) {
				match := describe(state.Repository.Identity.ID, view)
				match.Commit = state.Snapshot.Commit
				matches = append(matches, match)
			}
		}
		return nil, SearchOutput{Query: query, Matches: matches}, nil
	}
}

func snapshotForGet(catalog *serving.Catalog, access RepositoryAccess, named, file string) (serving.State, error) {
	states := accessibleStates(catalog, access)
	if named != "" {
		state, found := resolveState(states, named)
		if !found {
			return serving.State{}, fmt.Errorf("no accessible repository %q; served: %s", named, stateIDs(states))
		}
		if state.Snapshot == nil {
			return serving.State{}, fmt.Errorf("repository %s has no synchronized snapshot", state.Repository.Identity.ID)
		}
		return state, nil
	}
	if len(states) == 1 && states[0].Snapshot != nil {
		return states[0], nil
	}
	candidates := make([]serving.State, 0, len(states))
	for _, state := range states {
		if state.Snapshot == nil {
			continue
		}
		if _, found := state.Snapshot.File(file); found {
			candidates = append(candidates, state)
		}
	}
	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		return serving.State{}, fmt.Errorf("no repository has annotations for %s; served: %s", file, stateIDs(states))
	default:
		return serving.State{}, fmt.Errorf("%s is annotated in more than one repository (%s); pass repository to choose", file, stateIDs(candidates))
	}
}

func accessibleStates(catalog *serving.Catalog, access RepositoryAccess) []serving.State {
	states := catalog.States()
	if access == nil {
		return states
	}
	filtered := states[:0]
	for _, state := range states {
		if access[state.Repository.Identity.ID] {
			filtered = append(filtered, state)
		}
	}
	return filtered
}

func resolveState(states []serving.State, reference string) (serving.State, bool) {
	for _, state := range states {
		identity := state.Repository.Identity
		if identity.ID == reference || identity.Name != "" && strings.EqualFold(identity.Name, reference) {
			return state, true
		}
	}
	return serving.State{}, false
}

func stateIDs(states []serving.State) string {
	ids := make([]string, len(states))
	for index, state := range states {
		ids[index] = state.Repository.Identity.ID
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}

func displayIdentity(identity application.RepositoryIdentity) string {
	if identity.Name != "" {
		return identity.Name
	}
	return identity.ID
}

func validSnapshotPath(given string) (string, error) {
	path := strings.TrimSpace(given)
	if path == "" || path == "." || !fs.ValidPath(path) {
		return "", fmt.Errorf("file %q is not a repository-relative path", given)
	}
	return path, nil
}
