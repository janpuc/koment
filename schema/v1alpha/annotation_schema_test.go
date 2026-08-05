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
  "apiVersion": "koment.dev/v1alpha",
  "kind": "Annotation",
  "metadata": {
    "id": "01JQ8ZK3M4N5P6R7S8T9V0W1X2",
    "created": "2026-08-03T09:15:00Z"
  },
  "spec": {
    "target": {
      "file": "internal/session/token.go"
    },
    "type": "invariant",
    "body": "Keep the prior key through the rotation window.",
    "anchor": {
      "scope": "excerpt",
      "excerpt": "if token.Expired(now) {"
    },
    "author": {
      "name": "Fixture Author",
      "kind": "human",
      "source": "explicit"
    }
  },
  "status": {
    "lastSeenLine": 42
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

func specOf(record map[string]any) map[string]any {
	spec, _ := record["spec"].(map[string]any)
	return spec
}

func TestAnnotationSchemaAcceptsTheCurrentShape(t *testing.T) {
	if err := resolveAnnotationSchema(t).Validate(decodeRecord(t, validRecord)); err != nil {
		t.Fatalf("validate record: %v", err)
	}
}

func TestAnnotationSchemaAcceptsAPersistedObservation(t *testing.T) {
	record := decodeRecord(t, validRecord)
	record["status"] = map[string]any{
		"lastSeenLine":   float64(42),
		"resolution":     "ok",
		"resolvedAt":     "2026-08-05T11:00:00Z",
		"resolvedCommit": "0123456789abcdef0123456789abcdef01234567",
	}
	if err := resolveAnnotationSchema(t).Validate(record); err != nil {
		t.Fatalf("validate observation: %v", err)
	}
}

func TestAnnotationSchemaAcceptsAFileScopedRecordWithNoObservedLine(t *testing.T) {
	record := decodeRecord(t, validRecord)
	specOf(record)["anchor"] = map[string]any{"scope": "file"}
	record["status"] = map[string]any{
		"resolution":     "ok",
		"resolvedAt":     "2026-08-05T11:00:00Z",
		"resolvedCommit": "0123456789abcdef0123456789abcdef01234567",
	}
	if err := resolveAnnotationSchema(t).Validate(record); err != nil {
		t.Fatalf("validate file-scoped record: %v", err)
	}

	delete(record, "status")
	if err := resolveAnnotationSchema(t).Validate(record); err != nil {
		t.Fatalf("validate file-scoped record with no status at all: %v", err)
	}
}

func TestEveryCommittedAnnotationMatchesThePublishedSchema(t *testing.T) {
	resolved := resolveAnnotationSchema(t)
	patterns := []string{
		filepath.Join("..", "..", ".koment", "annotations", "*.yaml"),
		filepath.Join("..", "..", "workspace", ".koment", "annotations", "*.yaml"),
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
	spec := specOf(record)
	spec["type"] = "why"
	spec["policy"] = map[string]any{
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
		{"the retired version field", func(record map[string]any) { record["version"] = float64(1) }},
		{"a later api version", func(record map[string]any) { record["apiVersion"] = "koment.dev/v1" }},
		{"another resource kind", func(record map[string]any) { record["kind"] = "Rationale" }},
		{"unknown field", func(record map[string]any) { record["legacy"] = true }},
		{"unknown spec field", func(record map[string]any) { specOf(record)["confidence"] = "high" }},
		{"a bare file outside target", func(record map[string]any) {
			specOf(record)["file"] = "internal/session/token.go"
		}},
		{"a date instead of an instant", func(record map[string]any) {
			record["metadata"] = map[string]any{"id": "01JQ8ZK3M4N5P6R7S8T9V0W1X2", "created": "2026-08-03"}
		}},
		{"absolute path", func(record map[string]any) {
			specOf(record)["target"] = map[string]any{"file": "/etc/passwd"}
		}},
		{"noncanonical path", func(record map[string]any) {
			specOf(record)["target"] = map[string]any{"file": "./main.go"}
		}},
		{"backslash path", func(record map[string]any) {
			specOf(record)["target"] = map[string]any{"file": `internal\main.go`}
		}},
		{"drive-relative path", func(record map[string]any) {
			specOf(record)["target"] = map[string]any{"file": "C:main.go"}
		}},
		{"an excerpt anchor with no observed line", func(record map[string]any) {
			delete(record, "status")
		}},
		{"a line on a file anchor", func(record map[string]any) {
			specOf(record)["anchor"] = map[string]any{"scope": "file"}
		}},
		{"a resolution with no time", func(record map[string]any) {
			record["status"] = map[string]any{"lastSeenLine": float64(42), "resolution": "ok"}
		}},
		{"a retired resolution", func(record map[string]any) {
			record["status"] = map[string]any{
				"lastSeenLine": float64(42), "resolution": "moved", "resolvedAt": "2026-08-05T11:00:00Z",
			}
		}},
		{"an abbreviated resolved commit", func(record map[string]any) {
			record["status"] = map[string]any{
				"lastSeenLine": float64(42), "resolution": "ok",
				"resolvedAt": "2026-08-05T11:00:00Z", "resolvedCommit": "0123456",
			}
		}},
		{"human migration source", func(record map[string]any) {
			specOf(record)["author"] = map[string]any{
				"name":   "Test Human",
				"kind":   "human",
				"source": "migration",
			}
		}},
		{"unacknowledged exception", func(record map[string]any) {
			specOf(record)["policy"] = map[string]any{
				"exception":    "inline-comment",
				"acknowledged": false,
			}
		}},
		{"non-why exception", func(record map[string]any) {
			spec := specOf(record)
			spec["type"] = "gotcha"
			spec["policy"] = map[string]any{
				"exception":    "inline-comment",
				"acknowledged": true,
			}
		}},
		{"file-wide exception", func(record map[string]any) {
			spec := specOf(record)
			spec["type"] = "why"
			spec["anchor"] = map[string]any{"scope": "file"}
			spec["policy"] = map[string]any{
				"exception":    "inline-comment",
				"acknowledged": true,
			}
			delete(record, "status")
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
