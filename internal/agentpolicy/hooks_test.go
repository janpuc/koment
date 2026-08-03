package agentpolicy

import (
	"strings"
	"testing"
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
	input := `{"tool_name":"apply_patch","tool_input":{"command":` + quotedJSON(t, patch) + `}}`
	output, err := PreToolOutput([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
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
	input := `{"tool_name":"apply_patch","tool_input":{"command":` + quotedJSON(t, patch) + `}}`
	output, err := PreToolOutput([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "{}\n" {
		t.Fatalf("output = %s", output)
	}
}

func quotedJSON(t *testing.T, value string) string {
	t.Helper()
	var encoded strings.Builder
	encoded.WriteByte('"')
	for _, character := range value {
		switch character {
		case '\\', '"':
			encoded.WriteByte('\\')
			encoded.WriteRune(character)
		case '\n':
			encoded.WriteString(`\n`)
		default:
			encoded.WriteRune(character)
		}
	}
	encoded.WriteByte('"')
	return encoded.String()
}
