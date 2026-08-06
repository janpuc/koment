package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/serving"
	"github.com/janpuc/koment/internal/store"
)

const (
	apiVersion             = "2022-11-28"
	maximumAPIResponse     = 16 << 20
	maximumAnnotation      = 1 << 20
	maximumSource          = 4 << 20
	maximumAnnotations     = 10_000
	maximumAnnotationBytes = 32 << 20
	maximumSourceBytes     = 128 << 20
	concurrentBlobReads    = 8
)

// Client reads immutable repository snapshots through GitHub's Git data API.
type Client struct {
	httpClient *http.Client
	endpoint   *url.URL
	token      string
}

func New(token string) *Client {
	endpoint, err := url.Parse("https://api.github.com/")
	if err != nil {
		panic(err)
	}
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		endpoint:   endpoint,
		token:      token,
	}
}

func newClient(endpoint *url.URL, httpClient *http.Client, token string) *Client {
	return &Client{endpoint: endpoint, httpClient: httpClient, token: token}
}

func (c *Client) Snapshot(ctx context.Context, repository serving.Repository) (*application.RepositorySnapshot, error) {
	if err := repository.Validate(); err != nil {
		return nil, err
	}
	owner, name, _ := strings.Cut(repository.Remote, "/")
	base := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)

	var reference struct {
		Object struct {
			SHA  string `json:"sha"`
			Type string `json:"type"`
		} `json:"object"`
	}
	if err := c.get(ctx, base+"/git/ref/heads/"+url.PathEscape(repository.Branch), &reference); err != nil {
		return nil, fmt.Errorf("resolving %s at %s: %w", repository.Remote, repository.Branch, err)
	}
	if reference.Object.Type != "commit" || !validObjectID(reference.Object.SHA) {
		return nil, fmt.Errorf("github ref %s/%s is not a full commit id", repository.Remote, repository.Branch)
	}

	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := c.get(ctx, base+"/git/commits/"+reference.Object.SHA, &commit); err != nil {
		return nil, fmt.Errorf("reading commit %s: %w", reference.Object.SHA, err)
	}
	if !validObjectID(commit.Tree.SHA) {
		return nil, fmt.Errorf("github commit %s has no full tree id", reference.Object.SHA)
	}

	trees := treeCache{client: c, base: base, entries: make(map[string][]treeEntry)}
	annotationTree, found, err := trees.directory(ctx, commit.Tree.SHA, []string{store.DirName, "annotations"})
	if err != nil {
		return nil, err
	}
	if !found {
		return application.AssembleSnapshot(application.SnapshotInput{
			Repository: identity(repository), Commit: reference.Object.SHA,
		})
	}
	entries, err := trees.load(ctx, annotationTree)
	if err != nil {
		return nil, err
	}
	requests := make([]blobRequest, 0, len(entries))
	for _, entry := range entries {
		if entry.Type == "tree" {
			return nil, fmt.Errorf("unexpected directory %s/%s in flat annotation store", store.DirName, entry.Path)
		}
		if entry.Type != "blob" || !strings.HasSuffix(entry.Path, ".yaml") {
			continue
		}
		requests = append(requests, blobRequest{SHA: entry.SHA, Size: entry.Size, Name: entry.Path})
	}
	if len(requests) > maximumAnnotations {
		return nil, fmt.Errorf("repository %s has %d annotations; limit is %d", repository.Identity.ID, len(requests), maximumAnnotations)
	}
	sort.Slice(requests, func(left, right int) bool { return requests[left].Name < requests[right].Name })
	annotationContent, err := c.blobs(ctx, base, requests, maximumAnnotation, maximumAnnotationBytes)
	if err != nil {
		return nil, err
	}
	records := make([]store.Annotation, 0, len(requests))
	for index, request := range requests {
		id := strings.TrimSuffix(request.Name, ".yaml")
		record, decodeErr := store.DecodeAnnotation(id, annotationContent[index])
		if decodeErr != nil {
			return nil, fmt.Errorf("decoding %s: %w", request.Name, decodeErr)
		}
		records = append(records, *record)
	}

	paths := uniqueSourcePaths(records)
	sourceRequests := make([]blobRequest, 0, len(paths))
	for _, sourcePath := range paths {
		entry, sourceFound, sourceErr := trees.file(ctx, commit.Tree.SHA, strings.Split(sourcePath, "/"))
		if sourceErr != nil {
			return nil, sourceErr
		}
		if sourceFound {
			sourceRequests = append(sourceRequests, blobRequest{
				SHA: entry.SHA, Size: entry.Size, Name: sourcePath,
			})
		}
	}
	sourceContent, err := c.blobs(ctx, base, sourceRequests, maximumSource, maximumSourceBytes)
	if err != nil {
		return nil, err
	}
	sources := make(map[string][]byte, len(sourceRequests))
	for index, request := range sourceRequests {
		sources[request.Name] = sourceContent[index]
	}
	return application.AssembleSnapshot(application.SnapshotInput{
		Repository: identity(repository), Commit: reference.Object.SHA,
		GeneratedAt: time.Now().UTC(), Records: records, Sources: sources,
	})
}

func identity(repository serving.Repository) application.RepositoryIdentity {
	identity := repository.Identity
	identity.CloneURL = "https://github.com/" + repository.Remote
	identity.DefaultBranch = repository.Branch
	return identity
}

func uniqueSourcePaths(records []store.Annotation) []string {
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		seen[record.Spec.Target.File] = struct{}{}
	}
	paths := make([]string, 0, len(seen))
	for sourcePath := range seen {
		paths = append(paths, sourcePath)
	}
	sort.Strings(paths)
	return paths
}

func validObjectID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	for _, character := range id {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

type treeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

type treeCache struct {
	client  *Client
	base    string
	mu      sync.Mutex
	entries map[string][]treeEntry
}

func (c *treeCache) load(ctx context.Context, sha string) ([]treeEntry, error) {
	c.mu.Lock()
	entries, found := c.entries[sha]
	c.mu.Unlock()
	if found {
		return entries, nil
	}
	var response struct {
		Entries   []treeEntry `json:"tree"`
		Truncated bool        `json:"truncated"`
	}
	if err := c.client.get(ctx, c.base+"/git/trees/"+sha, &response); err != nil {
		return nil, fmt.Errorf("reading tree %s: %w", sha, err)
	}
	if response.Truncated {
		return nil, fmt.Errorf("github truncated non-recursive tree %s", sha)
	}
	c.mu.Lock()
	if existing, loaded := c.entries[sha]; loaded {
		response.Entries = existing
	} else {
		c.entries[sha] = response.Entries
	}
	c.mu.Unlock()
	return response.Entries, nil
}

func (c *treeCache) directory(ctx context.Context, root string, parts []string) (string, bool, error) {
	current := root
	for _, part := range parts {
		entries, err := c.load(ctx, current)
		if err != nil {
			return "", false, err
		}
		entry, found := namedEntry(entries, part)
		if !found || entry.Type != "tree" {
			return "", false, nil
		}
		if !validObjectID(entry.SHA) {
			return "", false, fmt.Errorf("github tree entry %s has invalid id %q", part, entry.SHA)
		}
		current = entry.SHA
	}
	return current, true, nil
}

func (c *treeCache) file(ctx context.Context, root string, parts []string) (treeEntry, bool, error) {
	if len(parts) == 0 {
		return treeEntry{}, false, nil
	}
	current := root
	for index, part := range parts {
		entries, err := c.load(ctx, current)
		if err != nil {
			return treeEntry{}, false, err
		}
		entry, found := namedEntry(entries, part)
		if !found {
			return treeEntry{}, false, nil
		}
		if index == len(parts)-1 {
			return entry, entry.Type == "blob", nil
		}
		if entry.Type != "tree" {
			return treeEntry{}, false, nil
		}
		if !validObjectID(entry.SHA) {
			return treeEntry{}, false, fmt.Errorf("github tree entry %s has invalid id %q", part, entry.SHA)
		}
		current = entry.SHA
	}
	return treeEntry{}, false, nil
}

func namedEntry(entries []treeEntry, name string) (treeEntry, bool) {
	for _, entry := range entries {
		if entry.Path == name {
			return entry, true
		}
	}
	return treeEntry{}, false
}

type blobRequest struct {
	SHA  string
	Size int64
	Name string
}

func (c *Client) blobs(ctx context.Context, base string, requests []blobRequest, perBlob, totalLimit int64) ([][]byte, error) {
	for _, request := range requests {
		if !validObjectID(request.SHA) {
			return nil, fmt.Errorf("%s has invalid blob id %q", request.Name, request.SHA)
		}
		if request.Size < 0 || request.Size > perBlob {
			return nil, fmt.Errorf("%s is %d bytes; limit is %d", request.Name, request.Size, perBlob)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	content := make([][]byte, len(requests))
	jobs := make(chan int)
	var wait sync.WaitGroup
	var mu sync.Mutex
	var firstError error
	var total int64
	worker := func() {
		defer wait.Done()
		for index := range jobs {
			decoded, err := c.blob(ctx, base, requests[index], perBlob)
			mu.Lock()
			if err == nil {
				total += int64(len(decoded))
				if total > totalLimit {
					err = fmt.Errorf("snapshot content exceeds %d bytes", totalLimit)
				}
			}
			if err != nil && firstError == nil {
				firstError = err
				cancel()
			}
			if err == nil {
				content[index] = decoded
			}
			mu.Unlock()
		}
	}
	workers := min(concurrentBlobReads, len(requests))
	for range workers {
		wait.Add(1)
		go worker()
	}
sendJobs:
	for index := range requests {
		select {
		case jobs <- index:
		case <-ctx.Done():
			break sendJobs
		}
	}
	close(jobs)
	wait.Wait()
	if firstError != nil {
		return nil, firstError
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return content, nil
}

func (c *Client) blob(ctx context.Context, base string, request blobRequest, limit int64) ([]byte, error) {
	var response struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		Size     int64  `json:"size"`
	}
	if err := c.get(ctx, base+"/git/blobs/"+request.SHA, &response); err != nil {
		return nil, fmt.Errorf("reading %s: %w", request.Name, err)
	}
	if response.Encoding != "base64" {
		return nil, fmt.Errorf("reading %s: github returned encoding %q", request.Name, response.Encoding)
	}
	if response.Size < 0 || response.Size > limit {
		return nil, fmt.Errorf("%s is %d bytes; limit is %d", request.Name, response.Size, limit)
	}
	decoded, err := base64.StdEncoding.DecodeString(response.Content)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", request.Name, err)
	}
	if int64(len(decoded)) != response.Size || int64(len(decoded)) > limit {
		return nil, fmt.Errorf("reading %s: decoded size %d does not match declared size %d", request.Name, len(decoded), response.Size)
	}
	return decoded, nil
}

func (c *Client) get(ctx context.Context, apiPath string, target any) error {
	return c.requestJSON(ctx, http.MethodGet, apiPath, nil, target)
}

type apiError struct {
	status  int
	message string
	limit   RateLimit
}

func (e *apiError) Error() string {
	described := fmt.Sprintf("github returned %s: %s", http.StatusText(e.status), e.message)
	if budget := e.limit.String(); budget != "" {
		described += " (" + budget + ")"
	}
	return described
}

func hasStatus(err error, status int) bool {
	var responseError *apiError
	return errors.As(err, &responseError) && responseError.status == status
}

func (c *Client) requestJSON(ctx context.Context, method, apiPath string, input, target any) error {
	endpoint, err := url.Parse(strings.TrimRight(c.endpoint.String(), "/") + "/" + strings.TrimPrefix(apiPath, "/"))
	if err != nil {
		return err
	}
	var requestBody io.Reader
	if input != nil {
		encoded, encodeErr := json.Marshal(input)
		if encodeErr != nil {
			return fmt.Errorf("encoding github request: %w", encodeErr)
		}
		if len(encoded) > maximumAPIResponse {
			return fmt.Errorf("github request exceeds %d bytes", maximumAPIResponse)
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), requestBody)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("X-GitHub-Api-Version", apiVersion)
	request.Header.Set("User-Agent", "koment")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maximumAPIResponse+1))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		return errors.Join(readErr, closeErr)
	}
	if len(responseBody) > maximumAPIResponse {
		return fmt.Errorf("github response exceeds %d bytes", maximumAPIResponse)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var failure struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(responseBody, &failure)
		if failure.Message == "" {
			failure.Message = http.StatusText(response.StatusCode)
		}
		limit, _ := rateLimitFrom(response.Header)
		return &apiError{status: response.StatusCode, message: failure.Message, limit: limit}
	}
	if target == nil || len(responseBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return fmt.Errorf("decoding github response: %w", err)
	}
	return nil
}
