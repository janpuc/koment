package ui

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/koment-dev/koment/internal/serving"
)

func SnapshotHandler(catalog *serving.Catalog) http.Handler {
	return snapshotHandler(catalog, func(*http.Request) map[string]bool { return nil }, nil)
}

func SnapshotHandlerFor(catalog *serving.Catalog, access map[string]bool) http.Handler {
	return snapshotHandler(catalog, func(*http.Request) map[string]bool { return access }, nil)
}

func SnapshotHandlerAuthorized(catalog *serving.Catalog, access func(*http.Request) map[string]bool) http.Handler {
	return snapshotHandler(catalog, access, nil)
}

func SnapshotHandlerCapabilities(
	catalog *serving.Catalog, access func(*http.Request) map[string]bool,
	canWrite func(*http.Request, string) bool,
) http.Handler {
	return snapshotHandler(catalog, access, canWrite)
}

func snapshotHandler(
	catalog *serving.Catalog, access func(*http.Request) map[string]bool,
	canWrite func(*http.Request, string) bool,
) http.Handler {
	templates := template.Must(template.ParseFS(assets, "assets/*.html"))
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.FileServerFS(assets))
	mux.HandleFunc("GET /{$}", func(writer http.ResponseWriter, request *http.Request) {
		state, found := defaultSnapshotState(catalog, access(request))
		if !found {
			http.Error(writer, "no accessible repository is configured", http.StatusForbidden)
			return
		}
		http.Redirect(writer, request, repositoryPrefix+state.Repository.Identity.ID+"/", http.StatusFound)
	})
	mux.HandleFunc("GET "+repositoryPrefix+"{repository}/{$}", func(writer http.ResponseWriter, request *http.Request) {
		renderSnapshot(writer, templates, catalog, access(request), canWrite, request, request.PathValue("repository"), "")
	})
	mux.HandleFunc("GET "+repositoryPrefix+"{repository}/f/{path...}", func(writer http.ResponseWriter, request *http.Request) {
		renderSnapshot(writer, templates, catalog, access(request), canWrite, request, request.PathValue("repository"), request.PathValue("path"))
	})
	return mux
}

func renderSnapshot(
	writer http.ResponseWriter, templates *template.Template, catalog *serving.Catalog,
	access map[string]bool, canWrite func(*http.Request, string) bool, request *http.Request,
	repositoryID, requested string,
) {
	if access != nil && !access[repositoryID] {
		http.Error(writer, "repository access denied", http.StatusForbidden)
		return
	}
	state, found := catalog.State(repositoryID)
	if !found {
		http.Error(writer, fmt.Sprintf("no repository %q; serving: %s", repositoryID, strings.Join(catalog.IDs(), ", ")), http.StatusNotFound)
		return
	}
	if state.Snapshot == nil {
		http.Error(writer, fmt.Sprintf("repository %s has no synchronized snapshot", repositoryID), http.StatusServiceUnavailable)
		return
	}
	built, err := build(state.Snapshot, requested, servedLinks(repositoryID))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	built.Repository = state.Repository.Identity.Name
	if built.Repository == "" {
		built.Repository = repositoryID
	}
	built.Repositories = snapshotSwitcher(catalog, access, repositoryID)
	if canWrite != nil {
		built.CanWrite = canWrite(request, repositoryID)
	}
	built.CreatedID = request.URL.Query().Get("created")
	built.ReviewURL = request.URL.Query().Get("review")
	built.Snapshot = &snapshot{Commit: state.Snapshot.Commit, CommitURL: commitURL(state.Repository.Remote, state.Snapshot.Commit)}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("X-Koment-Commit", state.Snapshot.Commit)
	if err := templates.ExecuteTemplate(writer, "page.html", built); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func snapshotSwitcher(catalog *serving.Catalog, access map[string]bool, current string) []repositoryLink {
	repositories := accessibleRepositories(catalog, access)
	if len(repositories) < 2 {
		return nil
	}
	links := make([]repositoryLink, 0, len(repositories))
	for _, repository := range repositories {
		name := repository.Identity.Name
		if name == "" {
			name = repository.Identity.ID
		}
		links = append(links, repositoryLink{
			ID: repository.Identity.ID, Name: name,
			Href: repositoryPrefix + repository.Identity.ID + "/", Current: repository.Identity.ID == current,
		})
	}
	return links
}

func defaultSnapshotState(catalog *serving.Catalog, access map[string]bool) (serving.State, bool) {
	state, found := catalog.Default()
	if found && (access == nil || access[state.Repository.Identity.ID]) {
		return state, true
	}
	for _, candidate := range catalog.States() {
		if access == nil || access[candidate.Repository.Identity.ID] {
			return candidate, true
		}
	}
	return serving.State{}, false
}

func accessibleRepositories(catalog *serving.Catalog, access map[string]bool) []serving.Repository {
	repositories := catalog.Repositories()
	if access == nil {
		return repositories
	}
	filtered := repositories[:0]
	for _, repository := range repositories {
		if access[repository.Identity.ID] {
			filtered = append(filtered, repository)
		}
	}
	return filtered
}

func commitURL(remote, commit string) string {
	if remote == "" || commit == "" {
		return ""
	}
	return "https://github.com/" + remote + "/commit/" + commit
}
