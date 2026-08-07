package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/koment-dev/koment/internal/agentpolicy"
)

const preToolDescription = "Run the koment pre-tool hook against an edit or write intent. " +
	"Returns a decision: 'allow' when the proposed content does not add ordinary comment intent, " +
	"'deny' when it does, with the reason naming the offending file and line. " +
	"Use this from a client plugin to gate tool calls the same way `koment agents hook pre-tool` does."

type PreToolInput struct {
	ToolName string `json:"tool_name" jsonschema:"the agent's tool name; recognised values are apply_patch and opencode_edit"`
	FilePath string `json:"filePath,omitempty" jsonschema:"for opencode_edit, the path the tool would write"`
	Content  string `json:"content,omitempty" jsonschema:"for opencode_edit, the content the tool would write"`
	Command  string `json:"command,omitempty" jsonschema:"for apply_patch, the patch command body"`
}

type PreToolOutput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

func preTool(_ context.Context, _ *sdk.CallToolRequest, input PreToolInput) (*sdk.CallToolResult, PreToolOutput, error) {
	payload, err := json.Marshal(map[string]any{
		"tool_name": input.ToolName,
		"tool_input": map[string]any{
			"command":  input.Command,
			"filePath": input.FilePath,
			"content":  input.Content,
		},
	})
	if err != nil {
		return nil, PreToolOutput{}, fmt.Errorf("encoding pre-tool input: %w", err)
	}
	raw, err := agentpolicy.PreToolOutput(payload)
	if err != nil {
		return nil, PreToolOutput{}, err
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" {
		return nil, PreToolOutput{Decision: "allow"}, nil
	}
	var decoded struct {
		HookSpecificOutput struct {
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, PreToolOutput{}, fmt.Errorf("decoding pre-tool output: %w", err)
	}
	decision := "allow"
	reason := decoded.HookSpecificOutput.PermissionDecisionReason
	if strings.EqualFold(decoded.HookSpecificOutput.PermissionDecision, "deny") {
		decision = "deny"
	}
	return nil, PreToolOutput{Decision: decision, Reason: reason}, nil
}
