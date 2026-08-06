package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koment-dev/koment/internal/anchor"
	"github.com/koment-dev/koment/internal/application"
	"github.com/koment-dev/koment/internal/policy"
	"github.com/koment-dev/koment/internal/repository"
	"github.com/koment-dev/koment/internal/store"
)

func TestProtocolPresentsAnnotationsFromOpenContent(t *testing.T) {
	root, uri, source := lspRepository(t, "package sample\n\nfunc run() {\n\tretry()\n}\n")
	service := application.NewService(repository.Repository{ID: "sample", Root: root})
	_, err := service.Add(application.AddInput{
		File: "sample.go", Excerpt: "retry()", Kind: store.TypeWhy,
		Body:   "The upstream closes idle connections.",
		Author: store.Author{Name: "Fixture", Kind: store.AuthorHuman, Source: store.FromExplicit},
	})
	if err != nil {
		t.Fatal(err)
	}

	var input bytes.Buffer
	writeTestMessage(t, &input, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeTestMessage(t, &input, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "go", "version": 1, "text": source}},
	})
	writeTestMessage(t, &input, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "koment/annotations",
		"params": map[string]any{"textDocument": map[string]string{"uri": uri}},
	})
	writeTestMessage(t, &input, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "textDocument/hover",
		"params": map[string]any{"textDocument": map[string]string{"uri": uri}, "position": map[string]int{"line": 3, "character": 2}},
	})
	writeTestMessage(t, &input, map[string]any{"jsonrpc": "2.0", "id": 4, "method": "shutdown"})
	writeTestMessage(t, &input, map[string]any{"jsonrpc": "2.0", "method": "exit"})

	var output bytes.Buffer
	if err := Run(context.Background(), &input, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	messages := readTestMessages(t, &output)
	annotationResponse := responseByID(t, messages, "2")
	var items []annotationItem
	if err := json.Unmarshal(annotationResponse.Result, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Line != 4 || items[0].Body != "The upstream closes idle connections." {
		t.Fatalf("annotations = %#v", items)
	}
	hoverResponse := responseByID(t, messages, "3")
	if !strings.Contains(string(hoverResponse.Result), "upstream closes idle connections") {
		t.Fatalf("hover = %s", hoverResponse.Result)
	}
}

func TestExecuteConvertPersistsBeforeEditingSource(t *testing.T) {
	root, uri, source := lspRepository(t, "package sample\n\nfunc run() {\n\t// Retry because the peer closes idle connections.\n\tretry()\n}\n")
	var output bytes.Buffer
	server := &server{transport: newTransport(strings.NewReader(""), &output), documents: map[string]document{
		uri: {URI: uri, Content: []byte(source), Version: 1, Language: "go"},
	}}
	params, err := json.Marshal(executeCommandParams{
		Command: commandConvert,
		Arguments: []json.RawMessage{json.RawMessage(fmt.Sprintf(
			`{"uri":%q,"comment":%q}`, uri, "// Retry because the peer closes idle connections.",
		))},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, responseError, err := server.execute(params)
	if err != nil || responseError != nil {
		t.Fatalf("execute error = %v, response = %#v", err, responseError)
	}
	if result == nil {
		t.Fatal("execute returned no result")
	}
	content, err := os.ReadFile(filepath.Join(root, "sample.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "Retry because") {
		t.Fatalf("source still contains comment: %s", content)
	}
	records, err := store.Open(root).All()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Spec.Body != "Retry because the peer closes idle connections." {
		t.Fatalf("records = %#v", records)
	}
}

func lspRepository(t *testing.T, source string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, store.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := policy.Save(root, policy.Default()); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{
		{"init", "--initial-branch=main"},
		{"config", "user.name", "Fixture Author"},
		{"config", "user.email", "fixture@example.test"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(root, "sample.go"))}).String()
	return root, uri, source
}

func writeTestMessage(t *testing.T, writer io.Writer, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(body); err != nil {
		t.Fatal(err)
	}
}

func readTestMessages(t *testing.T, reader io.Reader) []rpcMessage {
	t.Helper()
	transport := newTransport(reader, io.Discard)
	var messages []rpcMessage
	for {
		message, err := transport.read()
		if errors.Is(err, io.EOF) {
			return messages
		}
		if err != nil {
			t.Fatal(err)
		}
		messages = append(messages, message)
	}
}

func responseByID(t *testing.T, messages []rpcMessage, id string) rpcMessage {
	t.Helper()
	for _, message := range messages {
		if string(message.ID) == id {
			return message
		}
	}
	t.Fatalf("response %s not found in %#v", id, messages)
	return rpcMessage{}
}

// A Go nil slice marshals to JSON null, and every list this server sends is
// declared by LSP as an array. A client that trusts the protocol calls .map on
// the result, so null reaches it as a TypeError rather than an empty view. The
// assertion has to read the wire, because the Go value is indistinguishable.
func TestListsReachTheWireAsArraysAndNeverAsNull(t *testing.T) {
	root, uri, source := lspRepository(t, "package sample\n\nfunc run() {\n\tretry()\n}\n")
	_ = root

	var input bytes.Buffer
	writeTestMessage(t, &input, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeTestMessage(t, &input, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "go", "version": 1, "text": source}},
	})
	writeTestMessage(t, &input, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "textDocument/codeAction",
		"params": map[string]any{
			"textDocument": map[string]string{"uri": uri},
			"range":        map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 0}},
			"context":      map[string]any{"diagnostics": []any{}},
		},
	})
	writeTestMessage(t, &input, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "textDocument/codeLens",
		"params": map[string]any{"textDocument": map[string]string{"uri": uri}},
	})
	writeTestMessage(t, &input, map[string]any{"jsonrpc": "2.0", "id": 4, "method": "shutdown"})
	writeTestMessage(t, &input, map[string]any{"jsonrpc": "2.0", "method": "exit"})

	var output bytes.Buffer
	if err := Run(context.Background(), &input, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	messages := readTestMessages(t, &output)

	for _, id := range []string{"2", "3"} {
		if got := string(responseByID(t, messages, id).Result); got == "null" {
			t.Errorf("response %s carries null; LSP declares it an array and a client calling .map on it throws", id)
		}
	}

	published := 0
	for _, message := range messages {
		if message.Method != "textDocument/publishDiagnostics" {
			continue
		}
		published++
		var params struct {
			Diagnostics json.RawMessage `json:"diagnostics"`
		}
		if err := json.Unmarshal(message.Params, &params); err != nil {
			t.Fatal(err)
		}
		if string(params.Diagnostics) == "null" {
			t.Error("publishDiagnostics carries null diagnostics; the editor client throws before it can render anything")
		}
	}
	if published == 0 {
		t.Fatal("no diagnostics were published, so this proves nothing")
	}
}

// A koment diagnostic must mean the build is red. An annotation whose excerpt
// simply moved down the file resolves uniquely and passes `koment check`, so
// marking it put a squiggle under healthy code (ADR 0114, ADR 0116).
func TestOnlyFailingStatusesBecomeDiagnostics(t *testing.T) {
	root, uri, source := lspRepository(t, "package sample\n\nfunc run() {\n\tretry()\n}\n")
	service := application.NewService(repository.Repository{ID: "sample", Root: root})
	if _, err := service.Add(application.AddInput{
		File: "sample.go", Excerpt: "retry()", Kind: store.TypeWhy, Body: "Upstream closes idle connections.",
		Author: store.Author{Name: "Fixture", Kind: store.AuthorHuman, Source: store.FromExplicit},
	}); err != nil {
		t.Fatal(err)
	}

	moved := "package sample\n\n// pushed down\n\nfunc run() {\n\tretry()\n}\n"
	var input bytes.Buffer
	writeTestMessage(t, &input, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	writeTestMessage(t, &input, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "go", "version": 1, "text": moved}},
	})
	writeTestMessage(t, &input, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "koment/annotations",
		"params": map[string]any{"textDocument": map[string]string{"uri": uri}},
	})
	writeTestMessage(t, &input, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "shutdown"})
	writeTestMessage(t, &input, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	_ = source

	var output bytes.Buffer
	if err := Run(context.Background(), &input, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	messages := readTestMessages(t, &output)

	var items []annotationItem
	if err := json.Unmarshal(responseByID(t, messages, "2").Result, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != string(anchor.StatusOK) {
		t.Fatalf("an annotation whose excerpt moved down the file still resolves cleanly: %#v", items)
	}
	const recordedLine, movedLine = 4, 6
	if items[0].Line != movedLine {
		t.Fatalf("this proves nothing unless the excerpt moved from line %d to %d, got %d",
			recordedLine, movedLine, items[0].Line)
	}

	for _, message := range messages {
		if message.Method != "textDocument/publishDiagnostics" {
			continue
		}
		var params struct {
			Diagnostics []diagnostic `json:"diagnostics"`
		}
		if err := json.Unmarshal(message.Params, &params); err != nil {
			t.Fatal(err)
		}
		for _, published := range params.Diagnostics {
			if published.Code == "koment.ok" || published.Code == "koment.moved" {
				t.Errorf("a healthy annotation is published as a diagnostic (%s); it passes koment check", published.Code)
			}
		}
	}
}
