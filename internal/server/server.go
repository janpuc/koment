package server

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/auth"
	"github.com/janpuc/koment/internal/config"
	githubprovider "github.com/janpuc/koment/internal/github"
	"github.com/janpuc/koment/internal/listen"
	"github.com/janpuc/koment/internal/mcp"
	"github.com/janpuc/koment/internal/metrics"
	"github.com/janpuc/koment/internal/serving"
	"github.com/janpuc/koment/internal/store"
	"github.com/janpuc/koment/internal/ui"
)

const (
	defaultAddress      = "127.0.0.1:8080"
	defaultSyncInterval = time.Minute
	initialSyncTimeout  = time.Minute
	shutdownGrace       = 10 * time.Second
	headerTimeout       = 10 * time.Second
	requestTimeout      = 2 * time.Minute
	maximumMCPRequest   = 1 << 20
	maximumMutationBody = 1 << 20
	minimumSyncInterval = 10 * time.Second
)

const usage = `koment serve presents authenticated human and agent views of Git repositories.

  koment serve --config <repositories.yaml> [--listen <addr>]

The UI, repository switcher and MCP endpoint read the same immutable commit
snapshots. Liveness and readiness are public; all source and rationale routes
require a trusted proxy identity, a scoped bearer credential, or a loopback
listener. Remote writes are available only when a GitHub token is configured.
`

func Serve(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(stderr, usage, "\nFlags (each also settable from the environment):\n", config.Usage(flags))
	}
	configurationPath := flags.String("config", "", "strict YAML repository configuration")
	address := flags.String("listen", defaultAddress, "authenticated UI and MCP listen address")
	metricsAddress := flags.String("metrics", "", "separate unauthenticated metrics address; off unless given")
	githubTokenFile := flags.String("github-token-file", "", "file containing the GitHub App or fine-grained token")
	credentialsFile := flags.String("credentials-file", "", "secret YAML file containing hashed scoped agent credentials")
	trustedProxies := flags.String("trusted-proxies", "", "comma-separated CIDR ranges allowed to assert human identity headers")
	humanWrites := flags.Bool("human-writes", false, "allow trusted-proxy humans to create reviewed annotations")
	syncInterval := flags.Duration("sync-interval", defaultSyncInterval, "repository refresh interval")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := config.FromEnvironment(flags); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("serve takes no arguments, got %s", flags.Arg(0))
	}
	if *configurationPath == "" {
		return errors.New("serve needs --config")
	}
	if *syncInterval < minimumSyncInterval {
		return fmt.Errorf("sync interval %s is below the %s minimum", *syncInterval, minimumSyncInterval)
	}

	repositories, err := loadRepositories(*configurationPath)
	if err != nil {
		return err
	}
	catalog, err := serving.NewCatalog(repositories)
	if err != nil {
		return err
	}
	resolvedAddress, err := listen.Address(*address)
	if err != nil {
		return err
	}
	proxyRanges, err := parsePrefixes(*trustedProxies)
	if err != nil {
		return err
	}
	credentials := auth.Credentials{}
	if *credentialsFile != "" {
		credentials, err = auth.LoadCredentials(*credentialsFile)
		if err != nil {
			return err
		}
	}
	allowLoopback := listen.IsLoopback(resolvedAddress)
	if !allowLoopback && len(proxyRanges) == 0 && len(credentials.Tokens) == 0 {
		return errors.New("a non-loopback server needs --trusted-proxies or --credentials-file")
	}
	authenticator, err := auth.New(auth.Configuration{
		AllowLoopback: allowLoopback, TrustedProxies: proxyRanges,
		HumanCanWrite: *humanWrites, CredentialStore: credentials,
	})
	if err != nil {
		return err
	}
	githubToken := ""
	if *githubTokenFile != "" {
		content, readErr := readBounded(*githubTokenFile, maximumConfiguration)
		if readErr != nil {
			return fmt.Errorf("reading GitHub token: %w", readErr)
		}
		githubToken = strings.TrimSpace(string(content))
		if githubToken == "" {
			return errors.New("GitHub token file is empty")
		}
	}
	provider := githubprovider.New(githubToken)
	synchronizer := serving.Synchronizer{Catalog: catalog, Source: provider}
	initialContext, cancelInitial := context.WithTimeout(context.Background(), initialSyncTimeout)
	if syncErr := synchronizer.RefreshAll(initialContext); syncErr != nil {
		fmt.Fprintf(stderr, "koment: initial synchronization incomplete: %v\n", syncErr)
	}
	cancelInitial()

	mainListener, err := net.Listen("tcp", resolvedAddress)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", resolvedAddress, err)
	}
	var metricsListener net.Listener
	if *metricsAddress != "" {
		metricsResolved, resolveErr := listen.Address(*metricsAddress)
		if resolveErr != nil {
			return errors.Join(resolveErr, mainListener.Close())
		}
		metricsListener, err = net.Listen("tcp", metricsResolved)
		if err != nil {
			return errors.Join(fmt.Errorf("listening for metrics on %s: %w", metricsResolved, err), mainListener.Close())
		}
	}

	rootContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(rootContext)
	defer cancel()
	recorder := metrics.Recorder(metrics.Discard{})
	var metricSet *metrics.Metrics
	if metricsListener != nil {
		metricSet = metrics.New()
		recorder = metricSet
	}
	go synchronize(ctx, synchronizer, *syncInterval, recorder, stderr)
	var materializer serving.Materializer
	if githubToken != "" {
		materializer = provider
	}
	application := &http.Server{
		Handler:           metrics.Instrument(recorder, "serve", Handler(catalog, authenticator, recorder, materializer)),
		ReadHeaderTimeout: headerTimeout, ReadTimeout: requestTimeout, WriteTimeout: requestTimeout,
	}
	var metricsServer *http.Server
	if metricsListener != nil {
		metricsServer = &http.Server{Handler: metricSet.Handler(), ReadHeaderTimeout: headerTimeout}
	}
	fmt.Fprintf(stderr, "koment: serving UI and MCP at http://%s\n", mainListener.Addr())
	if metricsListener != nil {
		fmt.Fprintf(stderr, "koment: metrics at http://%s/metrics\n", metricsListener.Addr())
	}
	return serveListeners(ctx, cancel, application, mainListener, metricsServer, metricsListener, stderr)
}

func Handler(
	catalog *serving.Catalog, authenticator *auth.Authenticator, recorder metrics.Recorder, materializer serving.Materializer,
) http.Handler {
	accessFromRequest := func(request *http.Request) map[string]bool {
		principal, found := auth.FromContext(request.Context())
		if !found {
			return map[string]bool{}
		}
		return repositoryAccess(catalog, principal, auth.Read)
	}
	canWrite := func(request *http.Request, repository string) bool {
		principal, found := auth.FromContext(request.Context())
		return found && materializer != nil && principal.Can(repository, auth.Write)
	}
	human := ui.SnapshotHandlerCapabilities(catalog, accessFromRequest, canWrite)
	agent := sdk.NewStreamableHTTPHandler(func(request *http.Request) *sdk.Server {
		principal, _ := auth.FromContext(request.Context())
		readAccess := repositoryAccess(catalog, principal, auth.Read)
		if materializer != nil && principal.Permissions[auth.Write] {
			return mcp.NewWritableSnapshotServer(catalog, recorder, mcp.RepositoryAccess(readAccess), principal.Author(), materializer)
		}
		return mcp.NewSnapshotServer(catalog, recorder, mcp.RepositoryAccess(readAccess))
	}, &sdk.StreamableHTTPOptions{
		Stateless: true, JSONResponse: true, MaxRequestBodyBytes: maximumMCPRequest,
		PropagateRequestCancellation: true,
	})
	protected := http.NewServeMux()
	protected.Handle("/mcp", agent)
	protected.HandleFunc("POST /r/{repository}/annotations", func(writer http.ResponseWriter, request *http.Request) {
		addRemoteAnnotation(writer, request, catalog, materializer)
	})
	protected.Handle("/", human)
	routes := http.NewServeMux()
	routes.HandleFunc("GET /livez", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(writer, "ok")
	})
	routes.HandleFunc("GET /readyz", func(writer http.ResponseWriter, _ *http.Request) {
		if err := catalog.Ready(); err != nil {
			http.Error(writer, err.Error(), http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(writer, "ready")
	})
	crossOrigin := http.NewCrossOriginProtection()
	routes.Handle("/", authenticator.Middleware(crossOrigin.Handler(protected)))
	return securityHeaders(routes)
}

func addRemoteAnnotation(
	writer http.ResponseWriter, request *http.Request, catalog *serving.Catalog, materializer serving.Materializer,
) {
	principal, authenticated := auth.FromContext(request.Context())
	repositoryID := request.PathValue("repository")
	if !authenticated || materializer == nil || !principal.Can(repositoryID, auth.Write) {
		http.Error(writer, "repository write access denied", http.StatusForbidden)
		return
	}
	state, found := catalog.State(repositoryID)
	if !found {
		http.Error(writer, "repository not found", http.StatusNotFound)
		return
	}
	if state.Snapshot == nil {
		http.Error(writer, "repository has no synchronized snapshot", http.StatusServiceUnavailable)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maximumMutationBody)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "invalid annotation form: "+err.Error(), http.StatusBadRequest)
		return
	}
	kind, err := store.ParseType(request.Form.Get("kind"))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	record, err := application.DraftAnnotation(state.Snapshot, application.AddInput{
		File: request.Form.Get("file"), Excerpt: request.Form.Get("excerpt"),
		Kind: kind, Body: request.Form.Get("body"), Author: principal.Author(),
	})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	review, err := materializer.Materialize(request.Context(), state.Repository, state.Snapshot.Commit, record)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadGateway)
		return
	}
	query := url.Values{"created": []string{record.Metadata.ID}, "review": []string{review.URL}}
	target := "/r/" + url.PathEscape(repositoryID) + "/f/" + escapeSourcePath(record.Spec.Target.File) + "?" + query.Encode()
	http.Redirect(writer, request, target, http.StatusSeeOther)
}

func escapeSourcePath(sourcePath string) string {
	parts := strings.Split(sourcePath, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func repositoryAccess(catalog *serving.Catalog, principal auth.Principal, permission auth.Permission) map[string]bool {
	access := make(map[string]bool)
	for _, repository := range catalog.Repositories() {
		if principal.Can(repository.Identity.ID, permission) {
			access[repository.Identity.ID] = true
		}
	}
	return access
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(writer, request)
	})
}

func synchronize(ctx context.Context, synchronizer serving.Synchronizer, interval time.Duration, recorder metrics.Recorder, stderr io.Writer) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	observeCatalog(synchronizer.Catalog, recorder, 0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			started := time.Now()
			if err := synchronizer.RefreshAll(ctx); err != nil {
				fmt.Fprintf(stderr, "koment: synchronization: %v\n", err)
			}
			observeCatalog(synchronizer.Catalog, recorder, time.Since(started))
		}
	}
}

func observeCatalog(catalog *serving.Catalog, recorder metrics.Recorder, duration time.Duration) {
	counts := make(map[anchor.Status]int)
	files := 0
	for _, state := range catalog.States() {
		if state.Snapshot == nil {
			continue
		}
		files += len(state.Snapshot.Files)
		for status, count := range state.Snapshot.Counts() {
			counts[status] += count
		}
	}
	recorder.ObserveRepository(counts, files, duration)
}

func serveListeners(
	ctx context.Context, cancel context.CancelFunc, application *http.Server, applicationListener net.Listener,
	metricsServer *http.Server, metricsListener net.Listener, stderr io.Writer,
) error {
	type result struct {
		name string
		err  error
	}
	listeners := 1
	results := make(chan result, 2)
	go func() { results <- result{name: "application", err: application.Serve(applicationListener)} }()
	if metricsServer != nil {
		listeners++
		go func() { results <- result{name: "metrics", err: metricsServer.Serve(metricsListener)} }()
	}
	go func() {
		<-ctx.Done()
		shutdownContext, stop := context.WithTimeout(context.Background(), shutdownGrace)
		defer stop()
		if err := application.Shutdown(shutdownContext); err != nil {
			fmt.Fprintf(stderr, "koment: shutting down application: %v\n", err)
		}
		if metricsServer != nil {
			if err := metricsServer.Shutdown(shutdownContext); err != nil {
				fmt.Fprintf(stderr, "koment: shutting down metrics: %v\n", err)
			}
		}
	}()
	first := <-results
	cancel()
	if first.err != nil && !errors.Is(first.err, http.ErrServerClosed) {
		return fmt.Errorf("%s listener: %w", first.name, first.err)
	}
	for range listeners - 1 {
		remaining := <-results
		if remaining.err != nil && !errors.Is(remaining.err, http.ErrServerClosed) {
			return fmt.Errorf("%s listener: %w", remaining.name, remaining.err)
		}
	}
	return nil
}

func parsePrefixes(specification string) ([]netip.Prefix, error) {
	if strings.TrimSpace(specification) == "" {
		return nil, nil
	}
	var prefixes []netip.Prefix
	for _, entry := range strings.Split(specification, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(entry))
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q is not a CIDR range: %w", entry, err)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}
