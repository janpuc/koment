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

	"github.com/janpuc/koment/internal/store"
)

const shutdownGrace = 5 * time.Second

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
	flags.Usage = func() { fmt.Fprint(stderr, transportUsage) }

	httpAddress := flags.String("http", "", "serve over HTTP at this address, with JSON responses")
	streamableAddress := flags.String("streamable-http", "", "serve over HTTP at this address, with SSE responses")
	if err := flags.Parse(args); err != nil {
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
	switch {
	case *httpAddress != "":
		return serveHTTP(ctx, annotations, *httpAddress, true, stderr)
	case *streamableAddress != "":
		return serveHTTP(ctx, annotations, *streamableAddress, false, stderr)
	}
	return newServer(annotations).Run(ctx, &sdk.StdioTransport{})
}

func openHere() (*store.Store, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("finding the working directory: %w", err)
	}
	root, err := store.FindRoot(workingDirectory)
	if err != nil {
		return nil, err
	}
	return store.Open(root), nil
}

func serveHTTP(ctx context.Context, annotations *store.Store, address string, jsonResponses bool, stderr io.Writer) error {
	resolved, err := loopbackByDefault(address)
	if err != nil {
		return err
	}
	warnIfReachableFromTheNetwork(resolved, stderr)

	handler := sdk.NewStreamableHTTPHandler(
		func(*http.Request) *sdk.Server { return newServer(annotations) },
		&sdk.StreamableHTTPOptions{JSONResponse: jsonResponses},
	)

	listener, err := net.Listen("tcp", resolved)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", resolved, err)
	}
	server := &http.Server{Handler: handler}

	go func() {
		<-ctx.Done()
		timeout, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		server.Shutdown(timeout)
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

// loopbackByDefault fills in a missing host with the loopback interface, so
// that a bare port never publishes the repository to the local network.
func loopbackByDefault(address string) (string, error) {
	if _, _, err := net.SplitHostPort(address); err != nil {
		if port, portErr := parseBarePort(address); portErr == nil {
			return net.JoinHostPort("127.0.0.1", port), nil
		}
		return "", fmt.Errorf("%q is not a valid address or port: %w", address, err)
	}

	host, port, _ := net.SplitHostPort(address)
	if host == "" {
		return net.JoinHostPort("127.0.0.1", port), nil
	}
	return address, nil
}

func parseBarePort(address string) (string, error) {
	if address == "" {
		return "", errors.New("no port given")
	}
	if _, err := net.LookupPort("tcp", address); err != nil {
		return "", err
	}
	return address, nil
}

func warnIfReachableFromTheNetwork(address string, stderr io.Writer) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return
	}
	if parsed := net.ParseIP(host); parsed != nil && parsed.IsLoopback() {
		return
	}
	if host == "localhost" {
		return
	}
	fmt.Fprintf(stderr,
		"koment: WARNING serving on %s, which is not loopback. There is no authentication; "+
			"anyone who can reach this port can read every annotation in the repository.\n", address)
}
