package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/janpuc/koment/internal/serving"
	"github.com/janpuc/koment/internal/store"
)

func (c *Client) Materialize(ctx context.Context, repository serving.Repository, baseCommit string, record store.Annotation) (serving.Materialization, error) {
	if err := repository.Validate(); err != nil {
		return serving.Materialization{}, err
	}
	if !validObjectID(baseCommit) {
		return serving.Materialization{}, fmt.Errorf("base commit %q is not a full object id", baseCommit)
	}
	encoded, err := store.EncodeAnnotation(&record)
	if err != nil {
		return serving.Materialization{}, err
	}
	if len(encoded) > maximumAnnotation {
		return serving.Materialization{}, fmt.Errorf("annotation %s is %d bytes; limit is %d", record.ID, len(encoded), maximumAnnotation)
	}
	owner, name, _ := strings.Cut(repository.Remote, "/")
	base := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
	branch := "koment/" + record.ID
	path := store.DirName + "/annotations/" + record.ID + ".yaml"

	branchCommit, exists, err := c.existingMaterialization(ctx, base, branch, path, encoded)
	if err != nil {
		return serving.Materialization{}, err
	}
	if !exists {
		branchCommit, err = c.createMaterializationCommit(ctx, base, baseCommit, branch, path, encoded, record)
		if err != nil {
			if !hasStatus(err, http.StatusUnprocessableEntity) {
				return serving.Materialization{}, err
			}
			branchCommit, exists, err = c.existingMaterialization(ctx, base, branch, path, encoded)
			if err != nil {
				return serving.Materialization{}, err
			}
			if !exists {
				return serving.Materialization{}, fmt.Errorf("github rejected branch %s but it does not contain annotation %s", branch, record.ID)
			}
		}
	}
	pullRequest, err := c.ensurePullRequest(ctx, base, owner, branch, repository.Branch, record)
	if err != nil {
		return serving.Materialization{}, err
	}
	return serving.Materialization{
		Branch: branch, Commit: branchCommit, PullRequest: pullRequest.Number, URL: pullRequest.URL,
	}, nil
}

func (c *Client) existingMaterialization(ctx context.Context, base, branch, path string, wanted []byte) (string, bool, error) {
	var reference struct {
		Object struct {
			SHA  string `json:"sha"`
			Type string `json:"type"`
		} `json:"object"`
	}
	err := c.get(ctx, base+"/git/ref/heads/"+url.PathEscape(branch), &reference)
	if hasStatus(err, http.StatusNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading materialization branch %s: %w", branch, err)
	}
	if reference.Object.Type != "commit" || !validObjectID(reference.Object.SHA) {
		return "", false, fmt.Errorf("materialization branch %s does not point to a full commit id", branch)
	}
	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := c.get(ctx, base+"/git/commits/"+reference.Object.SHA, &commit); err != nil {
		return "", false, fmt.Errorf("reading materialization commit %s: %w", reference.Object.SHA, err)
	}
	trees := treeCache{client: c, base: base, entries: make(map[string][]treeEntry)}
	entry, found, err := trees.file(ctx, commit.Tree.SHA, strings.Split(path, "/"))
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, fmt.Errorf("materialization branch %s exists without %s", branch, path)
	}
	content, err := c.blob(ctx, base, blobRequest{SHA: entry.SHA, Size: entry.Size, Name: path}, maximumAnnotation)
	if err != nil {
		return "", false, err
	}
	if !bytes.Equal(content, wanted) {
		return "", false, fmt.Errorf("materialization branch %s contains different content for %s", branch, path)
	}
	return reference.Object.SHA, true, nil
}

func (c *Client) createMaterializationCommit(
	ctx context.Context, base, parent, branch, path string, content []byte, record store.Annotation,
) (string, error) {
	var parentCommit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := c.get(ctx, base+"/git/commits/"+parent, &parentCommit); err != nil {
		return "", fmt.Errorf("reading base commit %s: %w", parent, err)
	}
	if !validObjectID(parentCommit.Tree.SHA) {
		return "", fmt.Errorf("base commit %s has no full tree id", parent)
	}
	var blob struct {
		SHA string `json:"sha"`
	}
	if err := c.requestJSON(ctx, http.MethodPost, base+"/git/blobs", map[string]any{
		"content": base64.StdEncoding.EncodeToString(content), "encoding": "base64",
	}, &blob); err != nil {
		return "", fmt.Errorf("creating annotation blob: %w", err)
	}
	if !validObjectID(blob.SHA) {
		return "", errorsForObject("annotation blob", blob.SHA)
	}
	var tree struct {
		SHA string `json:"sha"`
	}
	if err := c.requestJSON(ctx, http.MethodPost, base+"/git/trees", map[string]any{
		"base_tree": parentCommit.Tree.SHA,
		"tree":      []map[string]any{{"path": path, "mode": "100644", "type": "blob", "sha": blob.SHA}},
	}, &tree); err != nil {
		return "", fmt.Errorf("creating annotation tree: %w", err)
	}
	if !validObjectID(tree.SHA) {
		return "", errorsForObject("annotation tree", tree.SHA)
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if err := c.requestJSON(ctx, http.MethodPost, base+"/git/commits", map[string]any{
		"message": "koment: add " + record.ID, "tree": tree.SHA, "parents": []string{parent},
	}, &commit); err != nil {
		return "", fmt.Errorf("creating annotation commit: %w", err)
	}
	if !validObjectID(commit.SHA) {
		return "", errorsForObject("annotation commit", commit.SHA)
	}
	if err := c.requestJSON(ctx, http.MethodPost, base+"/git/refs", map[string]any{
		"ref": "refs/heads/" + branch, "sha": commit.SHA,
	}, nil); err != nil {
		return "", fmt.Errorf("creating annotation branch: %w", err)
	}
	return commit.SHA, nil
}

type pullRequest struct {
	Number int    `json:"number"`
	URL    string `json:"html_url"`
}

func (c *Client) ensurePullRequest(ctx context.Context, base, owner, branch, defaultBranch string, record store.Annotation) (pullRequest, error) {
	query := url.Values{
		"state": []string{"open"}, "head": []string{owner + ":" + branch}, "base": []string{defaultBranch}, "per_page": []string{"2"},
	}
	find := func() (pullRequest, bool, error) {
		var pulls []pullRequest
		if err := c.get(ctx, base+"/pulls?"+query.Encode(), &pulls); err != nil {
			return pullRequest{}, false, err
		}
		if len(pulls) > 1 {
			return pullRequest{}, false, fmt.Errorf("github returned several open pull requests for %s", branch)
		}
		if len(pulls) == 1 {
			return pulls[0], true, nil
		}
		return pullRequest{}, false, nil
	}
	if existing, found, err := find(); err != nil {
		return pullRequest{}, fmt.Errorf("finding pull request for %s: %w", branch, err)
	} else if found {
		return existing, nil
	}
	var created pullRequest
	err := c.requestJSON(ctx, http.MethodPost, base+"/pulls", map[string]any{
		"title": "koment: add rationale for " + record.File,
		"head":  branch, "base": defaultBranch,
		"body": "Adds koment annotation `" + record.ID + "` for `" + record.File + "`.",
	}, &created)
	if err == nil {
		if created.Number < 1 || created.URL == "" {
			return pullRequest{}, fmt.Errorf("github created an invalid pull request for %s", branch)
		}
		return created, nil
	}
	if !hasStatus(err, http.StatusUnprocessableEntity) {
		return pullRequest{}, fmt.Errorf("creating pull request for %s: %w", branch, err)
	}
	existing, found, findErr := find()
	if findErr != nil {
		return pullRequest{}, findErr
	}
	if !found {
		return pullRequest{}, fmt.Errorf("github rejected pull request for %s and no open pull request exists: %w", branch, err)
	}
	return existing, nil
}

func errorsForObject(name, id string) error {
	return fmt.Errorf("github returned invalid %s id %q", name, id)
}
