package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/koment-dev/koment/internal/metrics"
)

func serveOverHTTP(t *testing.T, jsonResponses bool) *httptest.Server {
	t.Helper()
	annotations := repositoryWithOneAnnotation(t)
	repositories := onlyRepository(t, annotations)

	handler := sdk.NewStreamableHTTPHandler(
		func(*http.Request) *sdk.Server { return newServer(repositories, metrics.Discard{}, false) },
		&sdk.StreamableHTTPOptions{JSONResponse: jsonResponses},
	)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func connectOverHTTP(t *testing.T, endpoint string) *sdk.ClientSession {
	t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "http-probe", Version: "0"}, nil)
	session, err := client.Connect(context.Background(),
		&sdk.StreamableClientTransport{Endpoint: endpoint, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatalf("connecting to %s: %v", endpoint, err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func TestStreamableHTTPServesAnnotations(t *testing.T) {
	server := serveOverHTTP(t, false)
	session := connectOverHTTP(t, server.URL)

	got := callTool[GetOutput](t, session, "koment_get", map[string]any{"file": "main.go"})
	if len(got.Annotations) != 1 {
		t.Fatalf("want 1 annotation, got %d", len(got.Annotations))
	}
	if got.Annotations[0].Status != "ok" {
		t.Errorf("want status ok, got %s", got.Annotations[0].Status)
	}
}

func TestPlainHTTPServesAnnotationsAsJSON(t *testing.T) {
	server := serveOverHTTP(t, true)
	session := connectOverHTTP(t, server.URL)

	got := callTool[SearchOutput](t, session, "koment_search", map[string]any{"query": "signalled"})
	if len(got.Matches) != 1 {
		t.Fatalf("want 1 match, got %d", len(got.Matches))
	}
}

func TestJSONResponseModeReturnsJSONContentType(t *testing.T) {
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":` +
		`{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"probe","version":"0"}}}`

	for _, mode := range []struct {
		name          string
		jsonResponses bool
		wantType      string
	}{
		{"json", true, "application/json"},
		{"streamable", false, "text/event-stream"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			server := serveOverHTTP(t, mode.jsonResponses)

			request, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(initialize))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")

			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()

			if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, mode.wantType) {
				t.Errorf("want Content-Type %s, got %s", mode.wantType, got)
			}
		})
	}
}

func TestServeRejectsBothTransportsAtOnce(t *testing.T) {
	var stderr strings.Builder
	err := Serve([]string{"--http", "8765", "--streamable-http", "8766"}, &stderr)
	if err == nil {
		t.Fatal("want an error when both transports are requested")
	}
	if !strings.Contains(err.Error(), "choose one") {
		t.Errorf("unhelpful error: %v", err)
	}
}

func TestServeRejectsWritesOverHTTP(t *testing.T) {
	var stderr strings.Builder
	err := Serve([]string{"--write", "--http", "8765"}, &stderr)
	if err == nil || !strings.Contains(err.Error(), "only over stdio") {
		t.Fatalf("err = %v", err)
	}
}
