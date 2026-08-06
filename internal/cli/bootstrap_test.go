package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapExplicitAgentsWritesPolicyAndSelectedAdaptersOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".koment"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	initGit(t, root)
	t.Chdir(root)

	var stdout, stderr strings.Builder
	env := Environment{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}
	code := Run([]string{"bootstrap", "--agents", "claude,opencode", "--non-interactive"}, env, Servers{})
	if code != ExitOK {
		t.Fatalf("bootstrap exited %d: %s%s", code, stdout.String(), stderr.String())
	}

	for _, wanted := range []string{".koment/policy.yaml", "CLAUDE.md", ".opencode/plugins/koment.js", "CONTRIBUTING.md"} {
		if !strings.Contains(stdout.String(), wanted) {
			t.Errorf("bootstrap output missing %q:\n%s", wanted, stdout.String())
		}
	}
	for _, unwanted := range []string{".cursor/mcp.json", ".codex/hooks.json", "AGENTS.md"} {
		if strings.Contains(stdout.String(), unwanted) {
			t.Errorf("unselected adapter was installed (%s):\n%s", unwanted, stdout.String())
		}
	}

	policy, err := os.ReadFile(filepath.Join(root, ".koment", "policy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(policy), "claude") || !strings.Contains(string(policy), "opencode") {
		t.Errorf("policy does not list selected adapters:\n%s", policy)
	}
}

func TestBootstrapPolicyOnlyDoesNotTouchAdapters(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".koment"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	initGit(t, root)
	t.Chdir(root)

	var stdout, stderr strings.Builder
	env := Environment{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}
	code := Run([]string{"bootstrap", "--policy-only", "--non-interactive"}, env, Servers{})
	if code != ExitOK {
		t.Fatalf("bootstrap exited %d: %s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), ".koment/policy.yaml") {
		t.Errorf("policy was not installed:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("AGENTS.md was installed with --policy-only")
	}
}

func TestBootstrapNonInteractiveErrorsWithoutSelection(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".koment"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	initGit(t, root)
	t.Chdir(root)

	var stdout, stderr strings.Builder
	env := Environment{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}
	code := Run([]string{"bootstrap", "--non-interactive"}, env, Servers{})
	if code != ExitUsage {
		t.Fatalf("bootstrap exited %d (want %d): %s%s", code, ExitUsage, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--agents") {
		t.Errorf("error does not mention --agents: %s", stderr.String())
	}
}

func TestBootstrapReusesExistingPolicyAdapterList(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".koment"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	initGit(t, root)
	t.Chdir(root)

	existing := `apiVersion: koment.dev/v1alpha
kind: Policy
spec:
  comments:
    mode: strict
    intrinsic: [toolchain-directive, generated-marker, upstream-link, deprecated, public-api]
  agents:
    adapters: [claude, opencode]
`
	if err := os.WriteFile(filepath.Join(root, ".koment", "policy.yaml"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	env := Environment{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}
	code := Run([]string{"bootstrap", "--non-interactive"}, env, Servers{})
	if code != ExitOK {
		t.Fatalf("bootstrap exited %d: %s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "CLAUDE.md") {
		t.Errorf("existing adapters were not reused:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), ".cursor/mcp.json") {
		t.Errorf("unselected adapter was installed:\n%s", stdout.String())
	}
}

func TestBootstrapRejectsUnknownAdapter(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".koment"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	initGit(t, root)
	t.Chdir(root)

	var stdout, stderr strings.Builder
	env := Environment{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}
	code := Run([]string{"bootstrap", "--agents", "claude,fly", "--non-interactive"}, env, Servers{})
	if code != ExitUsage {
		t.Fatalf("bootstrap exited %d (want %d): %s%s", code, ExitUsage, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "fly") {
		t.Errorf("error does not name the unknown adapter: %s", stderr.String())
	}
}
