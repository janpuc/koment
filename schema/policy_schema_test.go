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

//go:embed policy.schema.json
var policySchema []byte

func resolvePolicySchema(t *testing.T) *jsonschema.Resolved {
	t.Helper()
	var schema jsonschema.Schema
	if err := json.Unmarshal(policySchema, &schema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("resolve schema: %v", err)
	}
	return resolved
}

func decodePolicy(t *testing.T, content []byte) map[string]any {
	t.Helper()
	var yamlPolicy any
	if err := yaml.Unmarshal(content, &yamlPolicy); err != nil {
		t.Fatalf("decode policy: %v", err)
	}
	encoded, err := json.Marshal(yamlPolicy)
	if err != nil {
		t.Fatalf("convert policy to JSON: %v", err)
	}
	var policy map[string]any
	if err := json.Unmarshal(encoded, &policy); err != nil {
		t.Fatalf("decode converted policy: %v", err)
	}
	return policy
}

func TestCommittedPolicyMatchesPublishedSchema(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", ".koment", "policy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := resolvePolicySchema(t).Validate(decodePolicy(t, content)); err != nil {
		t.Fatalf("validate policy: %v", err)
	}
}

func TestPolicySchemaRejectsVersionTwoAndUnknownFields(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", ".koment", "policy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	resolved := resolvePolicySchema(t)
	for _, mutate := range []func(map[string]any){
		func(policy map[string]any) { policy["version"] = float64(2) },
		func(policy map[string]any) { policy["bypass"] = true },
	} {
		policy := decodePolicy(t, content)
		mutate(policy)
		if err := resolved.Validate(policy); err == nil {
			t.Fatal("invalid policy was accepted")
		}
	}
}
