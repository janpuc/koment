package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestServesOverRealStdio builds the binary and drives it as a subprocess,
// because the in-memory transport used elsewhere never exercises the stdio
// wiring that every agent actually connects through.
func TestServesOverRealStdio(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "koment")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building koment: %v\n%s", err, output)
	}

	repositoryRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	server := exec.Command(binary, "mcp")
	server.Dir = repositoryRoot

	ctx := context.Background()
	client := sdk.NewClient(&sdk.Implementation{Name: "stdio-probe", Version: "0"}, nil)
	session, err := client.Connect(ctx, &sdk.CommandTransport{Command: server}, nil)
	if err != nil {
		t.Fatalf("connecting over stdio: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	if got, want := strings.Join(names, ","), "koment_get,koment_repositories,koment_search"; got != want {
		t.Errorf("want tools %q, got %q", want, got)
	}

	result, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "koment_get",
		Arguments: map[string]any{"file": "internal/store/ulid.go"},
	})
	if err != nil {
		t.Fatalf("tools/call: %v", err)
	}
	if result.IsError {
		t.Fatalf("koment_get reported a tool error")
	}

	var payload struct {
		Annotations []struct {
			Kind   string `json:"kind"`
			Body   string `json:"body"`
			Status string `json:"status"`
		} `json:"annotations"`
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}

	if len(payload.Annotations) == 0 {
		t.Fatal("koment's own annotations were not served; DESIGN.md done-criterion 6 is not met")
	}
	if payload.Annotations[0].Status != "ok" {
		t.Errorf("want status ok, got %q", payload.Annotations[0].Status)
	}
	if !strings.Contains(payload.Annotations[0].Body, "Crockford") {
		t.Errorf("unexpected body: %q", payload.Annotations[0].Body)
	}
}
