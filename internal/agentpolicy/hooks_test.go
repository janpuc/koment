package agentpolicy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koment-dev/koment/internal/policy"
	"github.com/koment-dev/koment/internal/store"
)

func TestPreToolOutputFlagsCommentIntentButAllowsPublicDocumentation(t *testing.T) {
	patch := `*** Begin Patch
*** Update File: internal/sample.go
@@
+// Exported documents the API.
+func Exported() {}
+
+func internal() {
+    // Retry because the peer closes idle connections.
+    retry()
+}
*** End Patch`
	output := runPreTool(t, toolHookApplyPatch, toolHookPatch{Command: patch})
	if !strings.Contains(string(output), "Retry because") {
		t.Fatalf("output = %s", output)
	}
	if strings.Contains(string(output), "Exported documents") {
		t.Fatalf("public documentation was flagged: %s", output)
	}
	if !strings.Contains(string(output), `"permissionDecision":"deny"`) {
		t.Fatalf("comment intent was not blocked: %s", output)
	}
}

func TestPreToolOutputAllowsIntrinsicDirective(t *testing.T) {
	patch := "*** Begin Patch\n*** Update File: sample.go\n@@\n+//go:generate stringer -type=State\n*** End Patch"
	output := runPreTool(t, toolHookApplyPatch, toolHookPatch{Command: patch})
	if string(output) != "{}\n" {
		t.Fatalf("output = %s", output)
	}
}

func TestPreToolOutputOpencodeEditFlagsCommentIntent(t *testing.T) {
	file := "internal/sample.go"
	content := "// Exported documents the API.\nfunc Exported() {}\n\nfunc internal() {\n\t// Retry because the peer closes idle connections.\n\tretry()\n}\n"
	output := runPreTool(t, toolHookOpencodeEdit, toolHookPatch{FilePath: file, Content: content})
	if !strings.Contains(string(output), "Retry because") {
		t.Fatalf("output = %s", output)
	}
	if strings.Contains(string(output), "Exported documents") {
		t.Fatalf("public documentation was flagged: %s", output)
	}
	if !strings.Contains(string(output), `"permissionDecision":"deny"`) {
		t.Fatalf("comment intent was not blocked: %s", output)
	}
}

func TestPreToolOutputOpencodeEditIgnoresNonGo(t *testing.T) {
	output := runPreTool(t, toolHookOpencodeEdit, toolHookPatch{FilePath: "README.md", Content: "// not Go\n"})
	if string(output) != "{}\n" {
		t.Fatalf("output = %s", output)
	}
}

func TestPreToolOutputOpencodeEditAllowsIntrinsicDirective(t *testing.T) {
	output := runPreTool(t, toolHookOpencodeEdit, toolHookPatch{FilePath: "sample.go", Content: "//go:generate stringer -type=State\n"})
	if string(output) != "{}\n" {
		t.Fatalf("output = %s", output)
	}
}

func TestPreToolOutputAllowsCommentThatMatchesPolicyAnnotation(t *testing.T) {
	root := withPolicyContaining(t, "renovate[\\s:]")
	t.Chdir(root)

	patch := "*** Begin Patch\n*** Update File: internal/sample.go\n@@\n+// renovate: enable\n*** End Patch"
	output := runPreTool(t, toolHookApplyPatch, toolHookPatch{Command: patch})
	if string(output) != "{}\n" {
		t.Fatalf("output = %s", output)
	}
}

func TestPreToolOutputBlocksCommentWhenPatternDoesNotMatch(t *testing.T) {
	root := withPolicyContaining(t, "renovate[\\s:]")
	t.Chdir(root)

	patch := "*** Begin Patch\n*** Update File: internal/sample.go\n@@\n+// explain the call\n*** End Patch"
	output := runPreTool(t, toolHookApplyPatch, toolHookPatch{Command: patch})
	if !strings.Contains(string(output), `"permissionDecision":"deny"`) {
		t.Fatalf("ordinary comment was not blocked: %s", output)
	}
}

func withPolicyContaining(t *testing.T, pattern string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, store.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	configured := policy.Default()
	configured.Spec.Comments.AllowedAnnotations = []string{pattern}
	if err := policy.Save(root, configured); err != nil {
		t.Fatal(err)
	}
	return root
}

type toolHookPatch struct {
	Command  string
	FilePath string
	Content  string
}

func runPreTool(t *testing.T, tool string, patch toolHookPatch) []byte {
	t.Helper()
	payload := map[string]any{
		"tool_name":  tool,
		"tool_input": map[string]any{"command": patch.Command, "filePath": patch.FilePath, "content": patch.Content},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	output, err := PreToolOutput(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return output
}
