package agentpolicy

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/janpuc/koment/internal/policy"
)

const (
	managedStart = "<!-- koment:managed-start -->"
	managedEnd   = "<!-- koment:managed-end -->"
	configStart  = "# koment:managed-start"
	configEnd    = "# koment:managed-end"
)

// Change names one adapter written by Install.
type Change struct {
	Path   string
	Action string
}

// Drift names one missing or stale adapter contract.
type Drift struct {
	Path   string
	Reason string
}

// Contract is the mandatory repository procedure shared by agent clients.
func Contract() string {
	return `## koment procedure

- Before editing an existing file, call ` + "`koment_get`" + ` for it and read every annotation. Search with ` + "`koment_search`" + ` before changing a non-obvious decision.
- Treat ambiguous, drifted and orphaned annotations as history, never as current fact.
- Do not add explanatory inline comments. Prefer a better name, extraction, a named type or constant, or clearer structure; then record local rationale with ` + "`koment_add`" + ` and honest agent authorship.
- Completed comment intent must use ` + "`koment_convert_comment`" + `. Keeping an inline comment requires ` + "`koment_acknowledge_comment`" + ` with the explicit acknowledgement set to true.
- Before finishing, run ` + "`koment check`" + `, ` + "`koment comments check`" + ` and ` + "`koment agents check`" + `. Do not report success while any fails.`
}

// Install creates or refreshes every adapter selected by the policy.
func Install(rootPath string, configured policy.Policy) (_ []Change, returnedError error) {
	if err := configured.Validate(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("opening repository root %s: %w", rootPath, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			returnedError = errors.Join(returnedError, closeErr)
		}
	}()

	var changes []Change
	for _, adapter := range configured.Agents.Adapters {
		installed, err := installAdapter(root, adapter)
		if err != nil {
			return nil, err
		}
		changes = append(changes, installed...)
	}
	return changes, nil
}

// Check rejects an adapter that no longer contains the selected contract.
func Check(rootPath string, configured policy.Policy) (_ []Drift, returnedError error) {
	if err := configured.Validate(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("opening repository root %s: %w", rootPath, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			returnedError = errors.Join(returnedError, closeErr)
		}
	}()

	var drift []Drift
	for _, adapter := range configured.Agents.Adapters {
		found, err := adapterDrift(root, adapter)
		if err != nil {
			return nil, err
		}
		drift = append(drift, found...)
	}
	return drift, nil
}

func installAdapter(root *os.Root, adapter policy.Adapter) ([]Change, error) {
	switch adapter {
	case policy.AdapterAgents:
		return installFiles([]installation{{"AGENTS.md", func() (bool, error) {
			return installManagedBlock(root, "AGENTS.md", Contract())
		}}})
	case policy.AdapterClaude:
		return installFiles([]installation{
			{"CLAUDE.md", func() (bool, error) { return installClaudeImport(root) }},
			{".mcp.json", func() (bool, error) { return installMCPJSON(root, ".mcp.json", "mcpServers") }},
		})
	case policy.AdapterCopilot:
		return installFiles([]installation{
			{".github/copilot-instructions.md", func() (bool, error) {
				return installManagedBlock(root, ".github/copilot-instructions.md", Contract())
			}},
			{".vscode/mcp.json", func() (bool, error) { return installMCPJSON(root, ".vscode/mcp.json", "servers") }},
		})
	case policy.AdapterCursor:
		return installFiles([]installation{
			{".cursor/rules/koment.mdc", func() (bool, error) {
				return replaceWhenDifferent(root, ".cursor/rules/koment.mdc", cursorRule())
			}},
			{".cursor/mcp.json", func() (bool, error) { return installMCPJSON(root, ".cursor/mcp.json", "mcpServers") }},
		})
	case policy.AdapterCodex:
		return installFiles([]installation{
			{".codex/hooks.json", func() (bool, error) { return installCodexHooks(root) }},
			{".codex/config.toml", func() (bool, error) { return installCodexConfig(root) }},
		})
	default:
		return nil, fmt.Errorf("unsupported agent adapter %q", adapter)
	}
}

type installation struct {
	path    string
	install func() (bool, error)
}

func installFiles(files []installation) ([]Change, error) {
	var changes []Change
	for _, file := range files {
		changed, err := file.install()
		if err != nil {
			return nil, err
		}
		if changed {
			changes = append(changes, Change{Path: file.path, Action: "installed"})
		}
	}
	return changes, nil
}

type adapterCheck struct {
	path  string
	check func() (string, error)
}

func adapterDrift(root *os.Root, adapter policy.Adapter) ([]Drift, error) {
	var checks []adapterCheck
	switch adapter {
	case policy.AdapterAgents:
		checks = append(checks, adapterCheck{"AGENTS.md", func() (string, error) {
			return managedBlockDrift(root, "AGENTS.md", Contract())
		}})
	case policy.AdapterClaude:
		checks = append(checks,
			adapterCheck{"CLAUDE.md", func() (string, error) { return claudeDrift(root) }},
			adapterCheck{".mcp.json", func() (string, error) {
				return mcpJSONDrift(root, ".mcp.json", "mcpServers")
			}},
		)
	case policy.AdapterCopilot:
		checks = append(checks,
			adapterCheck{".github/copilot-instructions.md", func() (string, error) {
				return managedBlockDrift(root, ".github/copilot-instructions.md", Contract())
			}},
			adapterCheck{".vscode/mcp.json", func() (string, error) {
				return mcpJSONDrift(root, ".vscode/mcp.json", "servers")
			}},
		)
	case policy.AdapterCursor:
		checks = append(checks,
			adapterCheck{".cursor/rules/koment.mdc", func() (string, error) {
				return exactDrift(root, ".cursor/rules/koment.mdc", cursorRule())
			}},
			adapterCheck{".cursor/mcp.json", func() (string, error) {
				return mcpJSONDrift(root, ".cursor/mcp.json", "mcpServers")
			}},
		)
	case policy.AdapterCodex:
		checks = append(checks,
			adapterCheck{".codex/hooks.json", func() (string, error) { return codexHooksDrift(root) }},
			adapterCheck{".codex/config.toml", func() (string, error) { return codexConfigDrift(root) }},
		)
	default:
		return nil, fmt.Errorf("unsupported agent adapter %q", adapter)
	}
	var drift []Drift
	for _, check := range checks {
		reason, err := check.check()
		if err != nil {
			return nil, err
		}
		if reason != "" {
			drift = append(drift, Drift{Path: check.path, Reason: reason})
		}
	}
	return drift, nil
}

func installManagedBlock(root *os.Root, name, body string) (bool, error) {
	content, err := readOptional(root, name)
	if err != nil {
		return false, err
	}
	wanted := managedStart + "\n" + body + "\n" + managedEnd
	updated, err := replaceManagedBlock(content, wanted)
	if err != nil {
		return false, fmt.Errorf("updating %s: %w", name, err)
	}
	return replaceWhenDifferent(root, name, updated)
}

func replaceManagedBlock(content, wanted string) (string, error) {
	return replaceNamedBlock(content, managedStart, managedEnd, wanted)
}

func replaceNamedBlock(content, startMarker, endMarker, wanted string) (string, error) {
	start := strings.Index(content, startMarker)
	end := strings.Index(content, endMarker)
	switch {
	case start < 0 && end < 0:
		if strings.TrimSpace(content) == "" {
			return wanted + "\n", nil
		}
		return strings.TrimRight(content, "\n") + "\n\n" + wanted + "\n", nil
	case start < 0 || end < 0 || end < start:
		return "", fmt.Errorf("managed block markers are incomplete")
	case strings.Contains(content[start+len(startMarker):end], startMarker) ||
		strings.Contains(content[end+len(endMarker):], endMarker):
		return "", fmt.Errorf("managed block markers occur more than once")
	default:
		end += len(endMarker)
		return content[:start] + wanted + content[end:], nil
	}
}

func managedBlockDrift(root *os.Root, name, body string) (string, error) {
	content, err := readOptional(root, name)
	if err != nil {
		return "", err
	}
	wanted := managedStart + "\n" + body + "\n" + managedEnd
	start := strings.Index(content, managedStart)
	end := strings.Index(content, managedEnd)
	if start < 0 || end < start {
		return "managed koment contract is missing", nil
	}
	end += len(managedEnd)
	if content[start:end] != wanted {
		return "managed koment contract has drifted", nil
	}
	return "", nil
}

func installClaudeImport(root *os.Root) (bool, error) {
	content, err := readOptional(root, "CLAUDE.md")
	if err != nil {
		return false, err
	}
	if hasLine(content, "@AGENTS.md") {
		return false, nil
	}
	updated := strings.TrimRight(content, "\n")
	if updated != "" {
		updated += "\n\n"
	}
	updated += "@AGENTS.md\n"
	return replaceWhenDifferent(root, "CLAUDE.md", updated)
}

func claudeDrift(root *os.Root) (string, error) {
	content, err := readOptional(root, "CLAUDE.md")
	if err != nil {
		return "", err
	}
	if !hasLine(content, "@AGENTS.md") {
		return "does not import AGENTS.md", nil
	}
	return "", nil
}

func hasLine(content, wanted string) bool {
	for line := range strings.SplitSeq(content, "\n") {
		if strings.TrimSpace(line) == wanted {
			return true
		}
	}
	return false
}

func cursorRule() string {
	return "---\ndescription: Enforce the koment rationale procedure\nglobs:\nalwaysApply: true\n---\n\n" + Contract() + "\n"
}

func installMCPJSON(root *os.Root, name, serversKey string) (bool, error) {
	content, err := readOptional(root, name)
	if err != nil {
		return false, err
	}
	document, err := decodeJSONObject(name, content)
	if err != nil {
		return false, err
	}
	servers := map[string]json.RawMessage{}
	if raw, found := document[serversKey]; found {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return false, fmt.Errorf("parsing %s %s: %w", name, serversKey, err)
		}
	}
	koment, err := json.Marshal(map[string]any{"command": "koment", "args": []string{"mcp", "--write"}})
	if err != nil {
		return false, fmt.Errorf("encoding koment MCP configuration: %w", err)
	}
	servers["koment"] = koment
	encodedServers, err := json.Marshal(servers)
	if err != nil {
		return false, fmt.Errorf("encoding %s %s: %w", name, serversKey, err)
	}
	document[serversKey] = encodedServers
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encoding %s: %w", name, err)
	}
	return replaceWhenDifferent(root, name, string(encoded)+"\n")
}

func mcpJSONDrift(root *os.Root, name, serversKey string) (string, error) {
	content, err := readOptional(root, name)
	if err != nil {
		return "", err
	}
	document, err := decodeJSONObject(name, content)
	if err != nil {
		return "", err
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(document[serversKey], &servers); err != nil {
		return "writable koment MCP server is missing", nil
	}
	var actual any
	if err := json.Unmarshal(servers["koment"], &actual); err != nil {
		return "writable koment MCP server is missing", nil
	}
	wanted := map[string]any{"command": "koment", "args": []any{"mcp", "--write"}}
	if !equalJSON(actual, wanted) {
		return "writable koment MCP server is missing or stale", nil
	}
	return "", nil
}

func decodeJSONObject(name, content string) (map[string]json.RawMessage, error) {
	document := map[string]json.RawMessage{}
	if strings.TrimSpace(content) == "" {
		return document, nil
	}
	if err := json.Unmarshal([]byte(content), &document); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", name, err)
	}
	return document, nil
}

func installCodexConfig(root *os.Root) (bool, error) {
	content, err := readOptional(root, ".codex/config.toml")
	if err != nil {
		return false, err
	}
	wanted := configStart + "\n[mcp_servers.koment]\ncommand = \"koment\"\nargs = [\"mcp\", \"--write\"]\n" + configEnd
	updated, err := replaceNamedBlock(content, configStart, configEnd, wanted)
	if err != nil {
		return false, fmt.Errorf("updating .codex/config.toml: %w", err)
	}
	return replaceWhenDifferent(root, ".codex/config.toml", updated)
}

func codexConfigDrift(root *os.Root) (string, error) {
	content, err := readOptional(root, ".codex/config.toml")
	if err != nil {
		return "", err
	}
	wanted := configStart + "\n[mcp_servers.koment]\ncommand = \"koment\"\nargs = [\"mcp\", \"--write\"]\n" + configEnd
	start := strings.Index(content, configStart)
	end := strings.Index(content, configEnd)
	if start < 0 || end < start {
		return "writable koment MCP configuration is missing", nil
	}
	end += len(configEnd)
	if content[start:end] != wanted {
		return "writable koment MCP configuration has drifted", nil
	}
	return "", nil
}

func installCodexHooks(root *os.Root) (bool, error) {
	content, err := readOptional(root, ".codex/hooks.json")
	if err != nil {
		return false, err
	}
	document, err := decodeHooks(content)
	if err != nil {
		return false, err
	}
	for event, wanted := range codexHookGroups() {
		groups := withoutKomentHooks(document.Hooks[event])
		document.Hooks[event] = append(groups, wanted)
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encoding .codex/hooks.json: %w", err)
	}
	return replaceWhenDifferent(root, ".codex/hooks.json", string(encoded)+"\n")
}

type hooksDocument struct {
	Description string                      `json:"description,omitempty"`
	Hooks       map[string][]map[string]any `json:"hooks"`
	Extra       map[string]json.RawMessage  `json:"-"`
}

func (d hooksDocument) MarshalJSON() ([]byte, error) {
	fields := make(map[string]any, len(d.Extra)+2)
	for name, raw := range d.Extra {
		fields[name] = raw
	}
	if d.Description != "" {
		fields["description"] = d.Description
	}
	fields["hooks"] = d.Hooks
	return json.Marshal(fields)
}

func decodeHooks(content string) (hooksDocument, error) {
	document := hooksDocument{Description: "Repository hooks including the koment policy guardrails.", Hooks: map[string][]map[string]any{}, Extra: map[string]json.RawMessage{}}
	if strings.TrimSpace(content) == "" {
		return document, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return hooksDocument{}, fmt.Errorf("parsing .codex/hooks.json: %w", err)
	}
	if value, found := raw["description"]; found {
		if err := json.Unmarshal(value, &document.Description); err != nil {
			return hooksDocument{}, fmt.Errorf("parsing .codex/hooks.json description: %w", err)
		}
		delete(raw, "description")
	}
	if value, found := raw["hooks"]; found {
		if err := json.Unmarshal(value, &document.Hooks); err != nil {
			return hooksDocument{}, fmt.Errorf("parsing .codex/hooks.json hooks: %w", err)
		}
		delete(raw, "hooks")
	}
	document.Extra = raw
	return document, nil
}

func codexHookGroups() map[string]map[string]any {
	return map[string]map[string]any{
		"PreToolUse": {
			"matcher": "^apply_patch$",
			"hooks": []any{map[string]any{
				"type": "command", "command": "koment agents hook pre-tool", "timeout": 10,
				"statusMessage": "Checking for explanatory comments",
			}},
		},
		"Stop": {
			"hooks": []any{map[string]any{
				"type": "command", "command": "koment agents hook stop", "timeout": 30,
				"statusMessage": "Checking koment policy",
			}},
		},
	}
}

func withoutKomentHooks(groups []map[string]any) []map[string]any {
	kept := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		encoded, err := json.Marshal(group)
		if err == nil && strings.Contains(string(encoded), `"command":"koment agents hook `) {
			continue
		}
		kept = append(kept, group)
	}
	return kept
}

func codexHooksDrift(root *os.Root) (string, error) {
	content, err := readOptional(root, ".codex/hooks.json")
	if err != nil {
		return "", err
	}
	document, err := decodeHooks(content)
	if err != nil {
		return "", err
	}
	for event, wanted := range codexHookGroups() {
		found := false
		for _, group := range document.Hooks[event] {
			if equalJSON(group, wanted) {
				found = true
				break
			}
		}
		if !found {
			return event + " koment hook is missing or stale", nil
		}
	}
	return "", nil
}

func equalJSON(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func exactDrift(root *os.Root, name, wanted string) (string, error) {
	content, err := readOptional(root, name)
	if err != nil {
		return "", err
	}
	if content != wanted {
		return "generated adapter is missing or stale", nil
	}
	return "", nil
}

func replaceWhenDifferent(root *os.Root, name, wanted string) (bool, error) {
	current, err := readOptional(root, name)
	if err != nil {
		return false, err
	}
	if current == wanted {
		return false, nil
	}
	if err := writeAtomically(root, name, []byte(wanted)); err != nil {
		return false, err
	}
	return true, nil
}

func readOptional(root *os.Root, name string) (string, error) {
	content, err := root.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", name, err)
	}
	return string(content), nil
}

func writeAtomically(root *os.Root, name string, content []byte) error {
	if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", name, err)
	}
	mode := fs.FileMode(0o644)
	if information, err := root.Stat(name); err == nil {
		mode = information.Mode().Perm()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("reading permissions for %s: %w", name, err)
	}
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return fmt.Errorf("creating temporary name for %s: %w", name, err)
	}
	temporaryName := name + "." + hex.EncodeToString(entropy[:])
	temporary, err := root.OpenFile(temporaryName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("creating temporary file beside %s: %w", name, err)
	}
	if _, err := temporary.Write(content); err != nil {
		return errors.Join(fmt.Errorf("writing %s: %w", temporaryName, err), temporary.Close(), root.Remove(temporaryName))
	}
	if err := temporary.Close(); err != nil {
		return errors.Join(fmt.Errorf("closing %s: %w", temporaryName, err), root.Remove(temporaryName))
	}
	if err := root.Rename(temporaryName, name); err != nil {
		return errors.Join(fmt.Errorf("replacing %s: %w", name, err), root.Remove(temporaryName))
	}
	return nil
}
