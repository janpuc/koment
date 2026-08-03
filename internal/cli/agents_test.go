package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentsInstallAndCheck(t *testing.T) {
	root := repository(t)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Existing instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	installed := koment(t, "agents", "install")
	if installed.code != ExitOK {
		t.Fatalf("install exited %d: %s", installed.code, installed.output())
	}
	for _, wanted := range []string{".koment/policy.yaml", "AGENTS.md", ".codex/hooks.json"} {
		if !strings.Contains(installed.stdout, wanted) {
			t.Errorf("install output missing %q:\n%s", wanted, installed.stdout)
		}
	}
	checked := koment(t, "agents", "check")
	if checked.code != ExitOK {
		t.Fatalf("check exited %d: %s", checked.code, checked.output())
	}

	if err := os.WriteFile(filepath.Join(root, ".cursor", "rules", "koment.mdc"), []byte("drifted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drifted := koment(t, "agents", "check")
	if drifted.code != ExitFailure || !strings.Contains(drifted.stdout, ".cursor/rules/koment.mdc") {
		t.Fatalf("drift was not found: %s", drifted.output())
	}
}

func TestAgentsPreToolHookGuidesCommentConversion(t *testing.T) {
	repository(t)
	input := `{"tool_name":"apply_patch","tool_input":{"command":"*** Begin Patch\n*** Update File: main.go\n@@\n+// Explain the call.\n+serve()\n*** End Patch"}}`
	var stdout, stderr bytes.Buffer
	env := Environment{Stdin: strings.NewReader(input), Stdout: &stdout, Stderr: &stderr}
	code := Run([]string{"agents", "hook", "pre-tool"}, env, Servers{})
	if code != ExitOK || !strings.Contains(stdout.String(), "koment_convert_comment") {
		t.Fatalf("hook exited %d: %s%s", code, stdout.String(), stderr.String())
	}
}

func TestAgentsStopHookContinuesOnPolicyFailure(t *testing.T) {
	root := repository(t)
	if got := koment(t, "agents", "install"); got.code != ExitOK {
		t.Fatalf("install exited %d: %s", got.code, got.output())
	}
	writeCommentedSource(t, root)

	var stdout, stderr bytes.Buffer
	env := Environment{Stdin: strings.NewReader(`{"stop_hook_active":false}`), Stdout: &stdout, Stderr: &stderr}
	code := Run([]string{"agents", "hook", "stop"}, env, Servers{})
	if code != ExitOK || !strings.Contains(stdout.String(), `"decision":"block"`) || !strings.Contains(stdout.String(), "inline comments") {
		t.Fatalf("hook exited %d: %s%s", code, stdout.String(), stderr.String())
	}
}
