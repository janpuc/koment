package agentpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janpuc/koment/internal/policy"
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
	if len(changes) != 9 {
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
