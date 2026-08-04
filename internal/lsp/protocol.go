package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

const maxMessageBytes = 8 << 20

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type transport struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex
}

func newTransport(reader io.Reader, writer io.Writer) *transport {
	return &transport{reader: bufio.NewReader(reader), writer: writer}
}

func (t *transport) read() (rpcMessage, error) {
	length := -1
	for {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			return rpcMessage{}, err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			break
		}
		name, value, found := strings.Cut(line, ":")
		if !found {
			return rpcMessage{}, fmt.Errorf("malformed LSP header %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(value))
			if parseErr != nil || parsed < 0 || parsed > maxMessageBytes {
				return rpcMessage{}, fmt.Errorf("invalid Content-Length %q", strings.TrimSpace(value))
			}
			length = parsed
		}
	}
	if length < 0 {
		return rpcMessage{}, errors.New("LSP message has no Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(t.reader, body); err != nil {
		return rpcMessage{}, err
	}
	var message rpcMessage
	if err := json.Unmarshal(body, &message); err != nil {
		return rpcMessage{}, fmt.Errorf("decoding LSP message: %w", err)
	}
	return message, nil
}

func (t *transport) send(value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding LSP message: %w", err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, err := fmt.Fprintf(t.writer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = t.writer.Write(body)
	return err
}

func (t *transport) respond(id json.RawMessage, result any, responseError *rpcError) error {
	response := struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  any             `json:"result,omitempty"`
		Error   *rpcError       `json:"error,omitempty"`
	}{JSONRPC: "2.0", ID: id, Result: result, Error: responseError}
	return t.send(response)
}

func (t *transport) notify(method string, params any) error {
	return t.send(struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{JSONRPC: "2.0", Method: method, Params: params})
}
