package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/provenance"
	"github.com/janpuc/koment/internal/store"
)

const (
	commandAdd         = "koment.add"
	commandReanchor    = "koment.reanchor"
	commandConvert     = "koment.convertComment"
	commandAcknowledge = "koment.acknowledgeComment"
)

var errExit = errors.New("LSP client exited")

type server struct {
	transport *transport
	documents map[string]document
	shutdown  bool
}

// Serve runs koment's language server over standard input and output.
func Serve(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("lsp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("lsp takes no positional arguments")
	}
	return Run(context.Background(), os.Stdin, os.Stdout, stderr)
}

// Run serves one LSP connection until the client exits or the context ends.
func Run(ctx context.Context, input io.Reader, output io.Writer, stderr io.Writer) error {
	server := &server{transport: newTransport(input, output), documents: map[string]document{}}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		message, err := server.transport.read()
		if errors.Is(err, io.EOF) || errors.Is(err, errExit) {
			return nil
		}
		if err != nil {
			return err
		}
		result, responseError, handleErr := server.handle(message)
		if errors.Is(handleErr, errExit) {
			return nil
		}
		if handleErr != nil {
			fmt.Fprintf(stderr, "koment lsp: %v\n", handleErr)
			responseError = &rpcError{Code: -32603, Message: handleErr.Error()}
		}
		if len(message.ID) > 0 {
			if err := server.transport.respond(message.ID, result, responseError); err != nil {
				return err
			}
		}
	}
}

func (s *server) handle(message rpcMessage) (any, *rpcError, error) {
	switch message.Method {
	case "initialize":
		return initializeResult(), nil, nil
	case "initialized", "$/cancelRequest", "workspace/didChangeConfiguration":
		return nil, nil, nil
	case "shutdown":
		s.shutdown = true
		return nil, nil, nil
	case "exit":
		return nil, nil, errExit
	case "textDocument/didOpen":
		return nil, nil, s.didOpen(message.Params)
	case "textDocument/didChange":
		return nil, nil, s.didChange(message.Params)
	case "textDocument/didSave":
		return nil, nil, s.didSave(message.Params)
	case "textDocument/didClose":
		return nil, nil, s.didClose(message.Params)
	case "textDocument/hover":
		return s.hover(message.Params)
	case "textDocument/codeLens":
		return s.codeLenses(message.Params)
	case "textDocument/codeAction":
		return s.codeActions(message.Params)
	case "workspace/executeCommand":
		return s.execute(message.Params)
	case "koment/annotations":
		return s.annotations(message.Params)
	default:
		if len(message.ID) == 0 {
			return nil, nil, nil
		}
		return nil, &rpcError{Code: -32601, Message: "method not found: " + message.Method}, nil
	}
}

func initializeResult() any {
	return map[string]any{
		"serverInfo": map[string]string{"name": "koment"},
		"capabilities": map[string]any{
			"textDocumentSync":   map[string]any{"openClose": true, "change": 1, "save": map[string]bool{"includeText": true}},
			"hoverProvider":      true,
			"codeLensProvider":   map[string]bool{"resolveProvider": false},
			"codeActionProvider": true,
			"executeCommandProvider": map[string]any{"commands": []string{
				commandAdd, commandReanchor, commandConvert, commandAcknowledge,
			}},
		},
	}
}

func (s *server) didOpen(raw json.RawMessage) error {
	var params struct {
		TextDocument struct {
			URI, LanguageID, Text string
			Version               int
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return err
	}
	s.documents[params.TextDocument.URI] = document{
		URI: params.TextDocument.URI, Content: []byte(params.TextDocument.Text),
		Version: params.TextDocument.Version, Language: params.TextDocument.LanguageID,
	}
	return s.publishDiagnostics(params.TextDocument.URI)
}

func (s *server) didChange(raw json.RawMessage) error {
	var params struct {
		TextDocument struct {
			URI     string `json:"uri"`
			Version int    `json:"version"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return err
	}
	if len(params.ContentChanges) != 1 {
		return fmt.Errorf("koment lsp requires one full-document content change")
	}
	doc := s.documents[params.TextDocument.URI]
	doc.URI = params.TextDocument.URI
	doc.Content = []byte(params.ContentChanges[0].Text)
	doc.Version = params.TextDocument.Version
	s.documents[doc.URI] = doc
	return s.publishDiagnostics(doc.URI)
}

func (s *server) didSave(raw json.RawMessage) error {
	var params struct {
		TextDocument textDocumentIdentifier `json:"textDocument"`
		Text         *string                `json:"text,omitempty"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return err
	}
	if params.Text != nil {
		doc := s.documents[params.TextDocument.URI]
		doc.URI = params.TextDocument.URI
		doc.Content = []byte(*params.Text)
		s.documents[doc.URI] = doc
	}
	return s.publishDiagnostics(params.TextDocument.URI)
}

func (s *server) didClose(raw json.RawMessage) error {
	var params struct {
		TextDocument textDocumentIdentifier `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return err
	}
	delete(s.documents, params.TextDocument.URI)
	return s.transport.notify("textDocument/publishDiagnostics", map[string]any{
		"uri": params.TextDocument.URI, "diagnostics": []diagnostic{},
	})
}

func (s *server) workspaceFile(uri string) (workspaceFile, error) {
	if opened, exists := s.documents[uri]; exists {
		return loadWorkspaceFile(uri, opened.Content)
	}
	return loadWorkspaceFile(uri, nil)
}

func (s *server) publishDiagnostics(uri string) error {
	file, err := s.workspaceFile(uri)
	if err != nil {
		return s.transport.notify("textDocument/publishDiagnostics", map[string]any{
			"uri": uri, "diagnostics": []diagnostic{},
		})
	}
	diagnostics, err := documentDiagnostics(file)
	if err != nil {
		diagnostics = []diagnostic{{
			Range: rangeValue{}, Severity: 1, Code: "koment.error", Source: "koment", Message: err.Error(),
		}}
	}
	return s.transport.notify("textDocument/publishDiagnostics", map[string]any{
		"uri": uri, "diagnostics": diagnostics,
	})
}

func (s *server) annotations(raw json.RawMessage) (any, *rpcError, error) {
	var params struct {
		TextDocument textDocumentIdentifier `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, nil, err
	}
	file, err := s.workspaceFile(params.TextDocument.URI)
	if err != nil {
		return []annotationItem{}, nil, nil
	}
	items, err := annotationItems(file)
	return items, nil, err
}

func (s *server) hover(raw json.RawMessage) (any, *rpcError, error) {
	var params textDocumentPositionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, nil, err
	}
	file, err := s.workspaceFile(params.TextDocument.URI)
	if err != nil {
		return nil, nil, nil
	}
	items, err := annotationItems(file)
	if err != nil {
		return nil, nil, err
	}
	var matched []string
	var matchedRange rangeValue
	for _, item := range items {
		if params.Position.Line < item.Range.Start.Line || params.Position.Line > item.Range.End.Line {
			continue
		}
		matched = append(matched, markdown(item))
		matchedRange = item.Range
	}
	if len(matched) == 0 {
		return nil, nil, nil
	}
	return map[string]any{
		"contents": map[string]string{"kind": "markdown", "value": strings.Join(matched, "\n\n---\n\n")},
		"range":    matchedRange,
	}, nil, nil
}

func (s *server) codeLenses(raw json.RawMessage) (any, *rpcError, error) {
	var params struct {
		TextDocument textDocumentIdentifier `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, nil, err
	}
	file, err := s.workspaceFile(params.TextDocument.URI)
	if err != nil {
		return []any{}, nil, nil
	}
	items, err := annotationItems(file)
	if err != nil {
		return nil, nil, err
	}
	lenses := make([]any, 0, len(items))
	for _, item := range items {
		title := item.Kind
		if item.Status != string(anchor.StatusOK) {
			title = fmt.Sprintf("%s · %s", item.Kind, item.Status)
		}
		lenses = append(lenses, map[string]any{
			"range":   item.Range,
			"command": command{Title: title, Command: "koment.showAnnotation", Arguments: []any{item}},
		})
	}
	return lenses, nil, nil
}

func (s *server) codeActions(raw json.RawMessage) (any, *rpcError, error) {
	var params struct {
		TextDocument textDocumentIdentifier `json:"textDocument"`
		Context      struct {
			Diagnostics []diagnostic `json:"diagnostics"`
		} `json:"context"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, nil, err
	}
	actions := []any{}
	for _, problem := range params.Context.Diagnostics {
		if problem.Code != "koment.comment" {
			continue
		}
		data, ok := problem.Data.(map[string]any)
		if !ok {
			continue
		}
		comment, _ := data["comment"].(string)
		arguments := mutationArguments{URI: params.TextDocument.URI, Comment: comment}
		actions = append(actions,
			map[string]any{"title": "Convert to koment", "kind": "quickfix", "command": command{Title: "Convert to koment", Command: commandConvert, Arguments: []any{arguments}}, "diagnostics": []diagnostic{problem}},
			map[string]any{"title": "Keep inline with acknowledgement", "kind": "quickfix", "command": command{Title: "Keep inline with acknowledgement", Command: commandAcknowledge, Arguments: []any{arguments}}, "diagnostics": []diagnostic{problem}},
		)
	}
	return actions, nil, nil
}

func (s *server) execute(raw json.RawMessage) (any, *rpcError, error) {
	var params executeCommandParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, nil, err
	}
	if len(params.Arguments) != 1 {
		return nil, nil, fmt.Errorf("%s requires one argument object", params.Command)
	}
	var arguments mutationArguments
	if err := json.Unmarshal(params.Arguments[0], &arguments); err != nil {
		return nil, nil, err
	}
	file, err := s.workspaceFile(arguments.URI)
	if err != nil {
		return nil, nil, err
	}
	if err := ensureSaved(file); err != nil {
		return nil, nil, err
	}
	author, err := provenance.IdentityFromGit(file.root)
	if err != nil && params.Command != commandReanchor {
		return nil, nil, err
	}
	kind := store.TypeWhy
	if arguments.Kind != "" {
		kind, err = store.ParseType(arguments.Kind)
		if err != nil {
			return nil, nil, err
		}
	}
	var mutation application.Mutation
	fileChanged := false
	switch params.Command {
	case commandAdd:
		mutation, err = file.service.Add(application.AddInput{
			File: file.relative, Excerpt: arguments.Excerpt, Kind: kind,
			Body: arguments.Body, Author: *author,
		})
	case commandReanchor:
		mutation, err = file.service.Reanchor(application.ReanchorInput{
			ID: arguments.ID, File: file.relative, Excerpt: arguments.Excerpt,
		})
	case commandConvert:
		mutation, err = file.service.ConvertComment(application.ConvertCommentInput{
			File: file.relative, Comment: arguments.Comment, Kind: kind, Author: *author,
		})
		fileChanged = err == nil
	case commandAcknowledge:
		mutation, err = file.service.AcknowledgeComment(application.AcknowledgeCommentInput{
			File: file.relative, Comment: arguments.Comment, Body: arguments.Body,
			Author: *author, Acknowledged: arguments.AcknowledgeInlineComment,
		})
	default:
		return nil, &rpcError{Code: -32602, Message: "unsupported command " + params.Command}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if fileChanged {
		if content, readErr := file.store.ReadSource(file.relative); readErr == nil {
			doc := s.documents[arguments.URI]
			doc.Content = content
			s.documents[arguments.URI] = doc
		}
	}
	if err := s.publishDiagnostics(arguments.URI); err != nil {
		return nil, nil, err
	}
	return map[string]any{
		"id": mutation.Record.Metadata.ID, "path": mutation.Path, "fileChanged": fileChanged,
		"warnings": mutation.Warnings,
	}, nil, nil
}

func ensureSaved(file workspaceFile) error {
	disk, err := file.store.ReadSource(file.relative)
	if err != nil {
		return err
	}
	if !bytes.Equal(disk, file.content) {
		return errors.New("save the document before changing koment annotations")
	}
	return nil
}
