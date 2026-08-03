package agentpolicy

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type toolHookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// PreToolOutput blocks a Go patch that adds ordinary comment intent.
func PreToolOutput(input []byte) ([]byte, error) {
	var request toolHookInput
	if err := json.Unmarshal(input, &request); err != nil {
		return nil, fmt.Errorf("parsing PreToolUse input: %w", err)
	}
	if request.ToolName != "apply_patch" {
		return []byte("{}\n"), nil
	}
	comments := addedCommentIntent(request.ToolInput.Command)
	if len(comments) == 0 {
		return []byte("{}\n"), nil
	}
	reason := fmt.Sprintf("koment policy blocked ordinary comment intent (%s). Record the rationale against nearby code with koment_add instead. If a comment already exists, use koment_convert_comment; retaining one requires koment_acknowledge_comment with explicit acknowledgement.", strings.Join(comments, ", "))
	response := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("encoding PreToolUse output: %w", err)
	}
	return append(encoded, '\n'), nil
}

// StopWasContinued reports whether the Stop hook already continued this turn.
func StopWasContinued(input []byte) (bool, error) {
	var request struct {
		StopHookActive bool `json:"stop_hook_active"`
	}
	if err := json.Unmarshal(input, &request); err != nil {
		return false, fmt.Errorf("parsing Stop input: %w", err)
	}
	return request.StopHookActive, nil
}

func addedCommentIntent(patch string) []string {
	lines := strings.Split(patch, "\n")
	file := ""
	var found []string
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		if name, ok := patchFile(line); ok {
			file = name
			continue
		}
		if !strings.HasSuffix(file, ".go") || !isAddedComment(line) {
			continue
		}
		start := index
		for index+1 < len(lines) && isAddedComment(lines[index+1]) {
			index++
		}
		group := lines[start : index+1]
		if intrinsicPatchComment(group) || publicDocumentationPatch(group, followingCode(lines, index+1)) {
			continue
		}
		found = append(found, fmt.Sprintf("%s: %s", file, strings.TrimSpace(strings.TrimPrefix(group[0], "+"))))
	}
	return found
}

func patchFile(line string) (string, bool) {
	prefixes := []string{"*** Add File: ", "*** Update File: "}
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return filepath.ToSlash(strings.TrimSpace(strings.TrimPrefix(line, prefix))), true
		}
	}
	return "", false
}

func isAddedComment(line string) bool {
	if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
		return false
	}
	trimmed := strings.TrimSpace(strings.TrimPrefix(line, "+"))
	return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*")
}

func intrinsicPatchComment(group []string) bool {
	raw := strings.Join(group, "\n")
	if strings.Contains(raw, "https://") || strings.Contains(raw, "http://") ||
		strings.Contains(raw, "Deprecated:") ||
		(strings.Contains(raw, "Code generated") && strings.Contains(raw, "DO NOT EDIT.")) {
		return true
	}
	for _, line := range group {
		text := strings.TrimSpace(strings.TrimPrefix(line, "+"))
		text = strings.TrimSpace(strings.TrimPrefix(text, "//"))
		allowed := false
		for _, prefix := range []string{"go:", "+build", "line ", "nolint", "lint:", "revive:", "gosec", "export ", "#cgo"} {
			if strings.HasPrefix(text, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}
	return true
}

func publicDocumentationPatch(group []string, code string) bool {
	first := strings.TrimSpace(strings.TrimPrefix(group[0], "+"))
	first = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(first, "//"), "/*"))
	name := strings.Fields(first)
	if len(name) == 0 {
		return false
	}
	if name[0] == "Package" && strings.HasPrefix(strings.TrimSpace(code), "package ") {
		return true
	}
	if name[0][0] < 'A' || name[0][0] > 'Z' {
		return false
	}
	quoted := regexp.QuoteMeta(strings.Trim(name[0], "`'\".,:;()"))
	declaration := regexp.MustCompile(`^(?:func\s+(?:\([^)]*\)\s+)?|type\s+|var\s+|const\s+)` + quoted + `\b`)
	return declaration.MatchString(strings.TrimSpace(code))
}

func followingCode(lines []string, start int) string {
	for _, line := range lines[start:] {
		if strings.HasPrefix(line, "*** ") || strings.HasPrefix(line, "@@") || strings.HasPrefix(line, "-") {
			continue
		}
		candidate := line
		if strings.HasPrefix(candidate, "+") || strings.HasPrefix(candidate, " ") {
			candidate = candidate[1:]
		}
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		return candidate
	}
	return ""
}
