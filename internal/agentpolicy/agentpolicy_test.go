package agentpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koment-dev/koment/internal/policy"
)

func TestInstallPreservesUnrelatedInstructionsAndHooks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Existing\n\nKeep this rule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	hooks := `{"custom":true,"hooks":{"Stop":[{"hooks":[{"type":"command","command":"custom check"}]}]}}`
	if err := os.WriteFile(filepath.Join(root, ".codex", "hooks.json"), []byte(hooks), 0o644); err != nil {
		t.Fatal(err)
	}

	changes, err := Install(root, policy.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 11 {
		t.Fatalf("changes = %#v", changes)
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), "Keep this rule.") || !strings.Contains(string(agents), Contract()) {
		t.Fatalf("AGENTS.md = %s", agents)
	}
	installedHooks, err := os.ReadFile(filepath.Join(root, ".codex", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{`"custom": true`, "custom check", "koment agents hook pre-tool", "koment agents hook stop"} {
		if !strings.Contains(string(installedHooks), wanted) {
			t.Errorf("hooks missing %q:\n%s", wanted, installedHooks)
		}
	}
	drift, err := Check(root, policy.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 0 {
		t.Fatalf("drift = %#v", drift)
	}
}

func TestCheckFindsChangedManagedContract(t *testing.T) {
	root := t.TempDir()
	if _, err := Install(root, policy.Default()); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(root, ".cursor", "rules", "koment.mdc")
	if err := os.WriteFile(name, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := Check(root, policy.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 1 || drift[0].Path != ".cursor/rules/koment.mdc" {
		t.Fatalf("drift = %#v", drift)
	}
}

func TestOpencodeAdapterInstallAndCheckRoundTrip(t *testing.T) {
	root := t.TempDir()
	changes, err := Install(root, policy.Default())
	if err != nil {
		t.Fatal(err)
	}
	pluginPath := filepath.Join(root, ".opencode", "plugins", "koment.js")
	if _, err := os.Stat(pluginPath); err != nil {
		t.Fatalf("plugin not installed: %v", err)
	}
	installed, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != opencodePlugin() {
		t.Fatalf("installed plugin differs from opencodePlugin():\n%s", installed)
	}
	opencodeConfig, err := os.ReadFile(filepath.Join(root, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(opencodeConfig), `"koment"`) {
		t.Fatalf("opencode.json missing koment MCP server: %s", opencodeConfig)
	}
	if !strings.Contains(string(opencodeConfig), "./.opencode/plugins/koment.js") {
		t.Fatalf("opencode.json missing plugin registration: %s", opencodeConfig)
	}
	if !containsChange(changes, ".opencode/plugins/koment.js") {
		t.Fatalf("changes missing plugin: %#v", changes)
	}
	if !containsChange(changes, "opencode.json") {
		t.Fatalf("changes missing config: %#v", changes)
	}
	drift, err := Check(root, policy.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 0 {
		t.Fatalf("drift = %#v", drift)
	}
}

func TestOpencodePluginPreservesUnrelatedJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "opencode.json"), []byte(`{"model":"anthropic/claude-sonnet-4-5","server":{"port":4096}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(root, policy.Default()); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(content)
	for _, kept := range []string{`"model": "anthropic/claude-sonnet-4-5"`, `"server":`, `"port": 4096`, `"koment"`, `"./.opencode/plugins/koment.js"`} {
		if !strings.Contains(encoded, kept) {
			t.Errorf("opencode.json missing %q:\n%s", kept, encoded)
		}
	}
}

func containsChange(changes []Change, path string) bool {
	for _, change := range changes {
		if change.Path == path {
			return true
		}
	}
	return false
}
