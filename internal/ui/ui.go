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
	"time"

	"github.com/janpuc/koment/internal/listen"
	"github.com/janpuc/koment/internal/store"
)

//go:embed assets
var assets embed.FS

const (
	defaultAddress = "127.0.0.1:0"
	shutdownGrace  = 5 * time.Second
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
	flags.Usage = func() { fmt.Fprint(stderr, usage) }

	address := flags.String("listen", defaultAddress, "address to serve on; a bare port is bound on loopback")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("ui takes no arguments, got %s", flags.Arg(0))
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("finding the working directory: %w", err)
	}
	root, err := store.FindRoot(workingDirectory)
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

	return serve(context.Background(), store.Open(root), listener)
}

func serve(ctx context.Context, annotations *store.Store, listener net.Listener) error {
	server := &http.Server{Handler: Handler(annotations)}

	go func() {
		<-ctx.Done()
		timeout, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		server.Shutdown(timeout)
	}()

	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Handler routes the view. Every request re-reads the working tree, so what is
// rendered is what is on disk rather than what was on disk at startup.
func Handler(annotations *store.Store) http.Handler {
	templates := template.Must(template.New("").Funcs(helpers).ParseFS(assets, "assets/*.html"))

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
	view, err := build(annotations, requested)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "page.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var helpers = template.FuncMap{
	"href": func(file string) string { return filePrefix + file },
}
