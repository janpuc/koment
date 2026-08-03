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
	"github.com/janpuc/koment/internal/repository"
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

	repositories, err := loadRepositories()
	if err != nil {
		return err
	}

	ctx := context.Background()
	recorder := startMetrics(ctx, repositories, *metricsAddress, stderr)

	switch {
	case *httpAddress != "":
		return serveHTTP(ctx, repositories, *httpAddress, true, stderr, recorder)
	case *streamableAddress != "":
		return serveHTTP(ctx, repositories, *streamableAddress, false, stderr, recorder)
	}
	return newServer(repositories, recorder).Run(ctx, &sdk.StdioTransport{})
}

func loadRepositories() (*repository.Set, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("finding the working directory: %w", err)
	}
	return repository.Load(workingDirectory)
}

// sweepAll reports the whole deployment. Gauges are summed across repositories
// because the metric answers "how much drift is there", which is a deployment
// question rather than a per-repository one.
func sweepAll(repositories *repository.Set, recorder metrics.Recorder) error {
	for _, entry := range repositories.All() {
		if err := metrics.Sweep(entry.Store(), recorder); err != nil {
			return err
		}
	}
	return nil
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
	go func() {
		ticker := time.NewTicker(sweepInterval)
		defer ticker.Stop()
		for {
			if err := sweepAll(repositories, recorder); err != nil {
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

func serveHTTP(ctx context.Context, repositories *repository.Set, address string, jsonResponses bool, stderr io.Writer, recorder metrics.Recorder) error {
	resolved, err := listen.Address(address)
	if err != nil {
		return err
	}
	listen.WarnIfPublic(resolved, stderr)

	handler := sdk.NewStreamableHTTPHandler(
		func(*http.Request) *sdk.Server { return newServer(repositories, recorder) },
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

	fmt.Fprintf(stderr, "koment: serving %d annotated files at http://%s\n", annotatedFileCount(repositories), listener.Addr())
	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func annotatedFileCount(repositories *repository.Set) int {
	total := 0
	for _, entry := range repositories.All() {
		files, err := entry.Store().AnnotatedFiles()
		if err != nil {
			continue
		}
		total += len(files)
	}
	return total
}
