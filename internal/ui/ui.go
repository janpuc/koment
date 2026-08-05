// Package ui serves a local view where code and its annotations converge on
// one screen.
package ui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/config"
	"github.com/janpuc/koment/internal/listen"
	"github.com/janpuc/koment/internal/metrics"
	"github.com/janpuc/koment/internal/provenance"
	"github.com/janpuc/koment/internal/repository"
	"github.com/janpuc/koment/internal/store"
)

//go:embed assets
var assets embed.FS

const (
	defaultAddress   = "127.0.0.1:0"
	shutdownGrace    = 5 * time.Second
	sweepInterval    = 30 * time.Second
	headerTimeout    = 10 * time.Second
	repositoryPrefix = "/r/"
	capabilityQuery  = "koment-capability"
	capabilityCookie = "koment_capability"
	maxMutationBody  = 1 << 20
)

const usage = `koment ui serves a local view of annotated code.

  koment ui [--listen <addr>] [--repository <id>] [--write]

Every configured repository is served, each under /r/<id>/, with a switcher on
the page. Pass --repository to serve only one.

<addr> may be a bare port. A host is added if omitted, and it is the loopback
interface: the view has no authentication, so anything that can reach the port
can read every annotation in every repository served.
`

// Serve parses the ui subcommand's own flags and runs the view until the
// process is interrupted.
func Serve(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("ui", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(stderr, usage, "\nFlags (each also settable from the environment):\n", config.Usage(flags))
	}

	address := flags.String("listen", defaultAddress, "address to serve on; a bare port is bound on loopback")
	metricsAddress := flags.String("metrics", "", "serve Prometheus metrics on this separate address; off unless given")
	named := flags.String("repository", "", "serve only this repository; all configured ones are served otherwise")
	writes := flags.Bool("write", false, "enable local annotation writes; valid only on loopback")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := config.FromEnvironment(flags); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("ui takes no arguments, got %s", flags.Arg(0))
	}

	repositories, err := selectedRepositories(*named)
	if err != nil {
		return err
	}

	resolved, err := listen.Address(*address)
	if err != nil {
		return err
	}
	listen.WarnIfPublic(resolved, stderr)
	if *writes && !listen.IsLoopback(resolved) {
		return fmt.Errorf("--write requires a loopback listen address")
	}
	writeToken := ""
	if *writes {
		if writeToken, err = newCapability(); err != nil {
			return err
		}
	}

	listener, err := net.Listen("tcp", resolved)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", resolved, err)
	}
	if writeToken == "" {
		fmt.Fprintf(stderr, "koment: http://%s\n", listener.Addr())
	} else {
		fmt.Fprintf(stderr, "koment: http://%s/?%s=%s\n", listener.Addr(), capabilityQuery, writeToken)
	}

	ctx := context.Background()
	recorder := startMetrics(ctx, repositories, *metricsAddress, stderr)

	return serve(ctx, repositories, listener, stderr, recorder, writeToken)
}

func selectedRepositories(named string) (*repository.Set, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("finding the working directory: %w", err)
	}
	repositories, err := repository.Load(workingDirectory)
	if err != nil {
		return nil, err
	}
	if named == "" {
		return repositories, nil
	}

	chosen, found := repositories.Resolve(named)
	if !found {
		return nil, fmt.Errorf("no repository %q; configured: %s",
			named, strings.Join(repositories.IDs(), ", "))
	}
	return repository.Of(chosen), nil
}

func startMetrics(ctx context.Context, repositories *repository.Set, address string, stderr io.Writer) metrics.Recorder {
	if address == "" {
		return metrics.Discard{}
	}

	recorder := metrics.New()
	go func() {
		if err := recorder.Serve(ctx, address, stderr); err != nil {
			fmt.Fprintf(stderr, "koment: metrics: %v\n", err)
		}
	}()
	go sweepPeriodically(ctx, repositories, recorder, stderr)
	return recorder
}

func sweepPeriodically(ctx context.Context, repositories *repository.Set, recorder metrics.Recorder, stderr io.Writer) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		for _, entry := range repositories.All() {
			if err := metrics.Sweep(entry.Store(), recorder); err != nil {
				fmt.Fprintf(stderr, "koment: metrics sweep: %s: %v\n", entry.ID, err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func serve(ctx context.Context, repositories *repository.Set, listener net.Listener, stderr io.Writer, recorder metrics.Recorder, writeToken string) error {
	server := &http.Server{
		Handler:           metrics.Instrument(recorder, "ui", handler(repositories, writeToken)),
		ReadHeaderTimeout: headerTimeout,
	}

	go func() {
		<-ctx.Done()
		timeout, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		if err := server.Shutdown(timeout); err != nil {
			fmt.Fprintf(stderr, "koment: shutting down: %v\n", err)
		}
	}()

	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Handler routes the view. Every request re-reads the working tree, so what is
// rendered is what is on disk rather than what was on disk at startup. Paths
// are /r/<repository>/f/<file>.
func Handler(repositories *repository.Set) http.Handler {
	return handler(repositories, "")
}

func handler(repositories *repository.Set, writeToken string) http.Handler {
	templates := template.Must(template.ParseFS(assets, "assets/*.html"))

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.FileServerFS(assets))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, repositoryPrefix+repositories.All()[0].ID+"/", http.StatusFound)
	})
	mux.HandleFunc("GET "+repositoryPrefix+"{repository}/{$}", func(w http.ResponseWriter, r *http.Request) {
		render(w, templates, repositories, r, r.PathValue("repository"), "", writeToken)
	})
	mux.HandleFunc("GET "+repositoryPrefix+"{repository}/f/{path...}", func(w http.ResponseWriter, r *http.Request) {
		render(w, templates, repositories, r, r.PathValue("repository"), r.PathValue("path"), writeToken)
	})
	if writeToken != "" {
		mux.HandleFunc("POST "+repositoryPrefix+"{repository}/annotations", func(w http.ResponseWriter, r *http.Request) {
			addFromBrowser(w, r, repositories, writeToken)
		})
	}
	return capabilityBootstrap(mux, writeToken)
}

func render(w http.ResponseWriter, templates *template.Template,
	repositories *repository.Set, request *http.Request, named, requested, writeToken string,
) {
	chosen, found := repositories.ByID(named)
	if !found {
		http.Error(w, fmt.Sprintf("no repository %q; serving: %s",
			named, strings.Join(repositories.IDs(), ", ")), http.StatusNotFound)
		return
	}

	repositorySnapshot, err := application.BuildSnapshot(chosen)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	view, err := build(repositorySnapshot, requested, servedLinks(chosen.ID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	view.Repository = chosen.Display()
	view.Repositories = repositorySwitcher(repositories, chosen.ID)
	if hasCapability(request, writeToken) {
		view.WriteToken = writeToken
		view.CanWrite = true
	}
	view.CreatedID = request.URL.Query().Get("created")
	view.WriteWarning = request.URL.Query().Get("warning")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "page.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func newCapability() (string, error) {
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("creating UI write capability: %w", err)
	}
	return hex.EncodeToString(entropy[:]), nil
}

func capabilityBootstrap(next http.Handler, writeToken string) http.Handler {
	if writeToken == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		given := r.URL.Query().Get(capabilityQuery)
		if r.Method == http.MethodGet && sameSecret(given, writeToken) {
			//nolint:gosec
			http.SetCookie(w, &http.Cookie{
				Name: capabilityCookie, Value: writeToken, Path: "/", HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func hasCapability(request *http.Request, writeToken string) bool {
	if writeToken == "" {
		return false
	}
	cookie, err := request.Cookie(capabilityCookie)
	return err == nil && sameSecret(cookie.Value, writeToken)
}

func sameSecret(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func addFromBrowser(w http.ResponseWriter, request *http.Request, repositories *repository.Set, writeToken string) {
	if !sameOrigin(request) || !hasCapability(request, writeToken) {
		http.Error(w, "write capability or same-origin request missing", http.StatusForbidden)
		return
	}
	request.Body = http.MaxBytesReader(w, request.Body, maxMutationBody)
	if err := request.ParseForm(); err != nil {
		http.Error(w, "invalid annotation form: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !sameSecret(request.Form.Get("capability"), writeToken) {
		http.Error(w, "CSRF token mismatch", http.StatusForbidden)
		return
	}
	entry, found := repositories.ByID(request.PathValue("repository"))
	if !found {
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}
	kind, err := store.ParseType(request.Form.Get("kind"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	author, err := provenance.IdentityFromGit(entry.Root)
	if err != nil {
		http.Error(w, "reading human identity: "+err.Error(), http.StatusBadRequest)
		return
	}
	mutation, err := application.NewService(entry).Add(application.AddInput{
		File: request.Form.Get("file"), Excerpt: request.Form.Get("excerpt"),
		Kind: kind, Body: request.Form.Get("body"), Author: *author,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query := url.Values{"created": []string{mutation.Record.Metadata.ID}}
	if len(mutation.Warnings) > 0 {
		query.Set("warning", strings.Join(mutation.Warnings, "; "))
	}
	target := repositoryPrefix + entry.ID + "/f/" + escapedFilePath(mutation.Record.Spec.Target.File) + "?" + query.Encode()
	http.Redirect(w, request, target, http.StatusSeeOther)
}

func sameOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Scheme == "http" && parsed.Host == request.Host
}

func repositorySwitcher(repositories *repository.Set, current string) []repositoryLink {
	if repositories.Len() < 2 {
		return nil
	}

	links := make([]repositoryLink, 0, repositories.Len())
	for _, entry := range repositories.All() {
		links = append(links, repositoryLink{
			ID:      entry.ID,
			Name:    entry.Display(),
			Href:    repositoryPrefix + entry.ID + "/",
			Current: entry.ID == current,
		})
	}
	return links
}
