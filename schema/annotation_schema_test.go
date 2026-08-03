package schema_test

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	yaml "go.yaml.in/yaml/v3"
)

//go:embed annotation.schema.json
var annotationSchema []byte

const validRecord = `{
  "version": 1,
  "id": "01JQ8ZK3M4N5P6R7S8T9V0W1X2",
  "file": "internal/session/token.go",
  "kind": "invariant",
  "body": "Keep the prior key through the rotation window.",
  "created": "2026-08-03",
  "anchor": {
    "scope": "excerpt",
    "excerpt": "if token.Expired(now) {",
    "last_seen_line": 42
  },
  "author": {
    "name": "Fixture Author",
    "kind": "human",
    "source": "explicit"
  }
}`

func resolveAnnotationSchema(t *testing.T) *jsonschema.Resolved {
	t.Helper()
	var schema jsonschema.Schema
	if err := json.Unmarshal(annotationSchema, &schema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("resolve schema: %v", err)
	}
	return resolved
}

func decodeRecord(t *testing.T, record string) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(record), &decoded); err != nil {
		t.Fatalf("decode record: %v", err)
	}
	return decoded
}

func TestAnnotationSchemaAcceptsVersionOne(t *testing.T) {
	if err := resolveAnnotationSchema(t).Validate(decodeRecord(t, validRecord)); err != nil {
		t.Fatalf("validate record: %v", err)
	}
}

func TestEveryCommittedAnnotationMatchesThePublishedSchema(t *testing.T) {
	resolved := resolveAnnotationSchema(t)
	patterns := []string{
		filepath.Join("..", ".koment", "annotations", "*.yaml"),
		filepath.Join("..", "workspace", ".koment", "annotations", "*.yaml"),
	}
	total := 0
	for _, pattern := range patterns {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			total++
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var yamlRecord any
			if err := yaml.Unmarshal(content, &yamlRecord); err != nil {
				t.Fatalf("decode %s: %v", path, err)
			}
			encoded, err := json.Marshal(yamlRecord)
			if err != nil {
				t.Fatalf("convert %s to JSON: %v", path, err)
			}
			var record map[string]any
			if err := json.Unmarshal(encoded, &record); err != nil {
				t.Fatalf("decode converted %s: %v", path, err)
			}
			if err := resolved.Validate(record); err != nil {
				t.Errorf("%s: %v", path, err)
			}
		}
	}
	if total == 0 {
		t.Fatal("no committed annotations were validated")
	}
}

func TestAnnotationSchemaAcceptsInlineCommentAcknowledgement(t *testing.T) {
	record := decodeRecord(t, validRecord)
	record["kind"] = "why"
	record["policy"] = map[string]any{
		"exception":    "inline-comment",
		"acknowledged": true,
	}
	if err := resolveAnnotationSchema(t).Validate(record); err != nil {
		t.Fatalf("validate acknowledgement: %v", err)
	}
}

func TestAnnotationSchemaRejectsUnsupportedRecords(t *testing.T) {
	resolved := resolveAnnotationSchema(t)
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"version two", func(record map[string]any) { record["version"] = float64(2) }},
		{"unknown field", func(record map[string]any) { record["legacy"] = true }},
		{"absolute path", func(record map[string]any) { record["file"] = "/etc/passwd" }},
		{"noncanonical path", func(record map[string]any) { record["file"] = "./main.go" }},
		{"backslash path", func(record map[string]any) { record["file"] = `internal\main.go` }},
		{"drive-relative path", func(record map[string]any) { record["file"] = "C:main.go" }},
		{"human migration source", func(record map[string]any) {
			record["author"] = map[string]any{
				"name":   "Test Human",
				"kind":   "human",
				"source": "migration",
			}
		}},
		{"unacknowledged exception", func(record map[string]any) {
			record["policy"] = map[string]any{
				"exception":    "inline-comment",
				"acknowledged": false,
			}
		}},
		{"non-why exception", func(record map[string]any) {
			record["kind"] = "gotcha"
			record["policy"] = map[string]any{
				"exception":    "inline-comment",
				"acknowledged": true,
			}
		}},
		{"file-wide exception", func(record map[string]any) {
			record["kind"] = "why"
			record["anchor"] = map[string]any{"scope": "file"}
			record["policy"] = map[string]any{
				"exception":    "inline-comment",
				"acknowledged": true,
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := decodeRecord(t, validRecord)
			test.mutate(record)
			if err := resolved.Validate(record); err == nil {
				t.Fatal("invalid record was accepted")
			}
		})
	}
}
