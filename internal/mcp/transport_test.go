package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func serveOverHTTP(t *testing.T, jsonResponses bool) *httptest.Server {
	t.Helper()
	annotations := repositoryWithOneAnnotation(t)

	handler := sdk.NewStreamableHTTPHandler(
		func(*http.Request) *sdk.Server { return newServer(annotations) },
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

func TestLoopbackByDefault(t *testing.T) {
	cases := map[string]string{
		"8765":            "127.0.0.1:8765",
		":8765":           "127.0.0.1:8765",
		"127.0.0.1:8765":  "127.0.0.1:8765",
		"0.0.0.0:8765":    "0.0.0.0:8765",
		"localhost:8765":  "localhost:8765",
		"192.168.1.5:900": "192.168.1.5:900",
	}

	for address, want := range cases {
		got, err := loopbackByDefault(address)
		if err != nil {
			t.Errorf("loopbackByDefault(%q): %v", address, err)
			continue
		}
		if got != want {
			t.Errorf("loopbackByDefault(%q) = %q, want %q", address, got, want)
		}
	}
}

func TestLoopbackByDefaultRejectsNonsense(t *testing.T) {
	for _, address := range []string{"not-a-port", "1:2:3", ""} {
		if got, err := loopbackByDefault(address); err == nil {
			t.Errorf("loopbackByDefault(%q) = %q, want an error", address, got)
		}
	}
}

func TestNonLoopbackBindIsWarnedAbout(t *testing.T) {
	var warning strings.Builder
	warnIfReachableFromTheNetwork("0.0.0.0:8765", &warning)
	if !strings.Contains(warning.String(), "WARNING") {
		t.Errorf("binding to all interfaces must warn, got %q", warning.String())
	}
	if !strings.Contains(warning.String(), "no authentication") {
		t.Errorf("the warning must say why it matters, got %q", warning.String())
	}

	var quiet strings.Builder
	warnIfReachableFromTheNetwork("127.0.0.1:8765", &quiet)
	if quiet.String() != "" {
		t.Errorf("loopback must not warn, got %q", quiet.String())
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
