package mcp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/janpuc/koment/internal/config"
	"github.com/janpuc/koment/internal/listen"
	"github.com/janpuc/koment/internal/metrics"
	"github.com/janpuc/koment/internal/store"
)

const (
	shutdownGrace = 5 * time.Second
	headerTimeout = 10 * time.Second
	sweepInterval = 30 * time.Second
)

const transportUsage = `koment mcp serves annotations to agents.

  koment mcp                            stdio (default)
  koment mcp --http <addr>              HTTP, JSON responses
  koment mcp --streamable-http <addr>   HTTP, server-sent events

<addr> may be a bare port. A host is added if omitted, and it is the loopback
interface: the server has no authentication, so anything that can reach the port
can read every annotation in the repository.
`

// Serve parses the mcp subcommand's own flags, so that package cli never links
// the MCP SDK.
func Serve(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("mcp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(stderr, transportUsage, "\nFlags (each also settable from the environment):\n", config.Usage(flags))
	}

	httpAddress := flags.String("http", "", "serve over HTTP at this address, with JSON responses")
	streamableAddress := flags.String("streamable-http", "", "serve over HTTP at this address, with SSE responses")
	metricsAddress := flags.String("metrics", "", "serve Prometheus metrics on this separate address; off unless given")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := config.FromEnvironment(flags); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("mcp takes no arguments, got %s", flags.Arg(0))
	}
	if *httpAddress != "" && *streamableAddress != "" {
		return errors.New("--http and --streamable-http are alternatives; choose one")
	}

	annotations, err := openHere()
	if err != nil {
		return err
	}

	ctx := context.Background()
	recorder := startMetrics(ctx, annotations, *metricsAddress, stderr)

	switch {
	case *httpAddress != "":
		return serveHTTP(ctx, annotations, *httpAddress, true, stderr, recorder)
	case *streamableAddress != "":
		return serveHTTP(ctx, annotations, *streamableAddress, false, stderr, recorder)
	}
	return newServer(annotations, recorder).Run(ctx, &sdk.StdioTransport{})
}

// repositoryRoot prefers KOMENT_REPO, because in a container the working
// directory is a mount point rather than somewhere a person navigated to.
func repositoryRoot() (string, error) {
	if root, ok := config.Root(); ok {
		return store.FindRoot(root)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("finding the working directory: %w", err)
	}
	return store.FindRoot(workingDirectory)
}

func openHere() (*store.Store, error) {
	root, err := repositoryRoot()
	if err != nil {
		return nil, err
	}
	return store.Open(root), nil
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
	go func() {
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
	}()
	return recorder
}

func serveHTTP(ctx context.Context, annotations *store.Store, address string, jsonResponses bool, stderr io.Writer, recorder metrics.Recorder) error {
	resolved, err := listen.Address(address)
	if err != nil {
		return err
	}
	listen.WarnIfPublic(resolved, stderr)

	handler := sdk.NewStreamableHTTPHandler(
		func(*http.Request) *sdk.Server { return newServer(annotations, recorder) },
		&sdk.StreamableHTTPOptions{JSONResponse: jsonResponses},
	)

	listener, err := net.Listen("tcp", resolved)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", resolved, err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: headerTimeout}

	go func() {
		<-ctx.Done()
		timeout, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		if err := server.Shutdown(timeout); err != nil {
			fmt.Fprintf(stderr, "koment: shutting down: %v\n", err)
		}
	}()

	fmt.Fprintf(stderr, "koment: serving %d annotated files at http://%s\n", annotatedFileCount(annotations), listener.Addr())
	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func annotatedFileCount(annotations *store.Store) int {
	files, err := annotations.AnnotatedFiles()
	if err != nil {
		return 0
	}
	return len(files)
}
