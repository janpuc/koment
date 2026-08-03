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
	"github.com/janpuc/koment/internal/store"
)

//go:embed assets
var assets embed.FS

const (
	defaultAddress = "127.0.0.1:0"
	shutdownGrace  = 5 * time.Second
	sweepInterval  = 30 * time.Second
	headerTimeout  = 10 * time.Second
	filePrefix     = "/f/"
)

const usage = `koment ui serves a local, read-only view of annotated code.

  koment ui [--listen <addr>]

<addr> may be a bare port. A host is added if omitted, and it is the loopback
interface: the view has no authentication, so anything that can reach the port
can read every annotation in the repository.
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
	named := flags.String("repository", "", "which repository to serve; required when several are configured")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := config.FromEnvironment(flags); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("ui takes no arguments, got %s", flags.Arg(0))
	}

	chosen, err := chooseRepository(*named)
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

	annotations := chosen.Store()
	ctx := context.Background()
	recorder := startMetrics(ctx, annotations, *metricsAddress, stderr)

	return serve(ctx, annotations, listener, stderr, recorder)
}

// startMetrics returns a no-op recorder unless an address was given, so the
// default posture exposes nothing new (ADR 0020).
// chooseRepository resolves which repository to serve. With several configured
// it refuses and lists them rather than picking one, matching koment_get's rule
// (ADR 0025). The UI serving all of them at once is PR 2's job.
func chooseRepository(named string) (repository.Repository, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return repository.Repository{}, fmt.Errorf("finding the working directory: %w", err)
	}
	repositories, err := repository.Load(workingDirectory)
	if err != nil {
		return repository.Repository{}, err
	}

	if named != "" {
		chosen, found := repositories.Resolve(named)
		if !found {
			return repository.Repository{}, fmt.Errorf("no repository %q; configured: %s",
				named, strings.Join(repositories.IDs(), ", "))
		}
		return chosen, nil
	}
	if only, single := repositories.Only(); single {
		return only, nil
	}
	return repository.Repository{}, fmt.Errorf(
		"%d repositories are configured (%s); pass --repository to choose one",
		repositories.Len(), strings.Join(repositories.IDs(), ", "))
}

func startMetrics(ctx context.Context, annotations *store.Store, address string, stderr io.Writer) metrics.Recorder {
	if address == "" {
		return metrics.Discard{}
	}

	recorder := metrics.New()
	go func() {
		if err := recorder.Serve(ctx, address, stderr); err != nil {
			fmt.Fprintf(stderr, "koment: metrics: %v\n", err)
		}
	}()
	go sweepPeriodically(ctx, annotations, recorder, stderr)
	return recorder
}

// sweepPeriodically keeps the repository gauges current. It is on a timer
// rather than driven by a scrape so that a scraper cannot drive load.
func sweepPeriodically(ctx context.Context, annotations *store.Store, recorder metrics.Recorder, stderr io.Writer) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		if err := metrics.Sweep(annotations, recorder); err != nil {
			fmt.Fprintf(stderr, "koment: metrics sweep: %v\n", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func serve(ctx context.Context, annotations *store.Store, listener net.Listener, stderr io.Writer, recorder metrics.Recorder) error {
	server := &http.Server{
		Handler:           metrics.Instrument(recorder, "ui", Handler(annotations)),
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
// rendered is what is on disk rather than what was on disk at startup.
func Handler(annotations *store.Store) http.Handler {
	templates := template.Must(template.ParseFS(assets, "assets/*.html"))

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.FileServerFS(assets))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		render(w, templates, annotations, "")
	})
	mux.HandleFunc("GET "+filePrefix+"{path...}", func(w http.ResponseWriter, r *http.Request) {
		render(w, templates, annotations, r.PathValue("path"))
	})
	return mux
}

func render(w http.ResponseWriter, templates *template.Template, annotations *store.Store, requested string) {
	view, err := build(annotations, requested, servedLinks())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "page.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
