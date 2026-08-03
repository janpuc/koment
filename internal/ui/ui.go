// Package ui serves a local read-only view where code and its annotations
// converge on one screen (ADR 0013).
package ui

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/janpuc/koment/internal/config"
	"github.com/janpuc/koment/internal/listen"
	"github.com/janpuc/koment/internal/metrics"
	"github.com/janpuc/koment/internal/repository"
)

//go:embed assets
var assets embed.FS

const (
	defaultAddress   = "127.0.0.1:0"
	shutdownGrace    = 5 * time.Second
	sweepInterval    = 30 * time.Second
	headerTimeout    = 10 * time.Second
	repositoryPrefix = "/r/"
)

const usage = `koment ui serves a local, read-only view of annotated code.

  koment ui [--listen <addr>] [--repository <id>]

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
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := config.FromEnvironment(flags); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("ui takes no arguments, got %s", flags.Arg(0))
	}

	repositories, err := serveable(*named)
	if err != nil {
		return err
	}

	resolved, err := listen.Address(*address)
	if err != nil {
		return err
	}
	listen.WarnIfPublic(resolved, stderr)

	listener, err := net.Listen("tcp", resolved)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", resolved, err)
	}
	fmt.Fprintf(stderr, "koment: http://%s\n", listener.Addr())

	ctx := context.Background()
	recorder := startMetrics(ctx, repositories, *metricsAddress, stderr)

	return serve(ctx, repositories, listener, stderr, recorder)
}

// serveable is the set the UI will serve: every configured repository, or the
// one --repository names.
func serveable(named string) (*repository.Set, error) {
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

// startMetrics returns a no-op recorder unless an address was given, so the
// default posture exposes nothing new (ADR 0020).
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

// sweepPeriodically keeps the repository gauges current. It is on a timer
// rather than driven by a scrape so that a scraper cannot drive load.
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

func serve(ctx context.Context, repositories *repository.Set, listener net.Listener, stderr io.Writer, recorder metrics.Recorder) error {
	server := &http.Server{
		Handler:           metrics.Instrument(recorder, "ui", Handler(repositories)),
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
	templates := template.Must(template.ParseFS(assets, "assets/*.html"))

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.FileServerFS(assets))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, repositoryPrefix+repositories.All()[0].ID+"/", http.StatusFound)
	})
	mux.HandleFunc("GET "+repositoryPrefix+"{repository}/{$}", func(w http.ResponseWriter, r *http.Request) {
		render(w, templates, repositories, r.PathValue("repository"), "")
	})
	mux.HandleFunc("GET "+repositoryPrefix+"{repository}/f/{path...}", func(w http.ResponseWriter, r *http.Request) {
		render(w, templates, repositories, r.PathValue("repository"), r.PathValue("path"))
	})
	return mux
}

func render(w http.ResponseWriter, templates *template.Template,
	repositories *repository.Set, named, requested string,
) {
	chosen, found := repositories.ByID(named)
	if !found {
		http.Error(w, fmt.Sprintf("no repository %q; serving: %s",
			named, strings.Join(repositories.IDs(), ", ")), http.StatusNotFound)
		return
	}

	view, err := build(chosen.Store(), requested, servedLinks(chosen.ID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	view.Repository = chosen.Display()
	view.Repositories = switcher(repositories, chosen.ID)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "page.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// switcher is empty for a single repository.
func switcher(repositories *repository.Set, current string) []repositoryLink {
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
