package lsp

import "encoding/json"

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type rangeValue struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type textDocumentPositionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type command struct {
	Title     string `json:"title"`
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}

type diagnostic struct {
	Range    rangeValue `json:"range"`
	Severity int        `json:"severity"`
	Code     string     `json:"code,omitempty"`
	Source   string     `json:"source"`
	Message  string     `json:"message"`
	Data     any        `json:"data,omitempty"`
}

type document struct {
	URI      string
	Content  []byte
	Version  int
	Language string
}

type executeCommandParams struct {
	Command   string            `json:"command"`
	Arguments []json.RawMessage `json:"arguments"`
}

type mutationArguments struct {
	URI                      string `json:"uri"`
	ID                       string `json:"id,omitempty"`
	Excerpt                  string `json:"excerpt,omitempty"`
	Kind                     string `json:"kind,omitempty"`
	Body                     string `json:"body,omitempty"`
	Comment                  string `json:"comment,omitempty"`
	AcknowledgeInlineComment bool   `json:"acknowledgeInlineComment,omitempty"`
}

type annotationItem struct {
	ID      string     `json:"id"`
	Kind    string     `json:"kind"`
	Body    string     `json:"body"`
	Status  string     `json:"status"`
	Line    int        `json:"line"`
	Warning string     `json:"warning,omitempty"`
	Range   rangeValue `json:"range"`
}
