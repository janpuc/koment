package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultRoundTripsAndMatchesExcludedPaths(t *testing.T) {
	root := t.TempDir()
	configured := Default()
	configured.Spec.Comments.GeneratedPaths = append(configured.Spec.Comments.GeneratedPaths, "generated/**")
	if err := Save(root, configured); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Excludes("nested/model.generated.go") || !loaded.Excludes("generated/deep/model.go") {
		t.Fatal("expected generated paths to be excluded")
	}
	if loaded.Excludes("internal/model.go") {
		t.Fatal("ordinary source was excluded")
	}
	content, err := os.ReadFile(filepath.Join(root, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(content), "# yaml-language-server: $schema="+SchemaURL) {
		t.Fatal("policy has no schema directive")
	}
}

func TestInstallPreservesExistingPolicy(t *testing.T) {
	root := t.TempDir()
	first, created, err := Install(root)
	if err != nil || !created {
		t.Fatalf("first install = %#v, %v, %v", first, created, err)
	}
	second, created, err := Install(root)
	if err != nil || created || second.APIVersion != APIVersion {
		t.Fatalf("second install = %#v, %v, %v", second, created, err)
	}
}

func TestPolicyRejectsBypassesAndUnknownValues(t *testing.T) {
	cases := map[string]func(*Policy){
		"non-strict mode":   func(value *Policy) { value.Spec.Comments.Mode = "off" },
		"escaping pattern":  func(value *Policy) { value.Spec.Comments.VendoredPaths = []string{"../vendor/**"} },
		"absolute pattern":  func(value *Policy) { value.Spec.Comments.VendoredPaths = []string{"/vendor/**"} },
		"unknown intrinsic": func(value *Policy) { value.Spec.Comments.Intrinsic = []Intrinsic{"anything"} },
		"unknown adapter":   func(value *Policy) { value.Spec.Agents.Adapters = []Adapter{"anything"} },
		"duplicate adapter": func(value *Policy) { value.Spec.Agents.Adapters = []Adapter{AdapterAgents, AdapterAgents} },
		"wrong api version": func(value *Policy) { value.APIVersion = "koment.dev/v1" },
		"wrong kind":        func(value *Policy) { value.Kind = "Configuration" },
		"unknown principle": func(value *Policy) { value.Spec.Agents.Principles = []Principle{"anything"} },
		"duplicate principle": func(value *Policy) {
			value.Spec.Agents.Principles = []Principle{PrincipleBackCompatEvidence, PrincipleBackCompatEvidence}
		},
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			configured := Default()
			corrupt(&configured)
			if err := configured.Validate(); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
}

func TestLoadUpgradesAFlatPolicyAndRewritesItInPlace(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".koment"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `version: 1
comments:
  mode: strict
  intrinsic:
    - toolchain-directive
  generated_paths:
    - '**/*.gen.go'
  vendored_paths:
    - vendor/**
agents:
  adapters:
    - agents
`
	path := filepath.Join(root, FileName)
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.APIVersion != APIVersion || loaded.Kind != KindPolicy {
		t.Fatalf("policy was not upgraded: %#v", loaded)
	}
	if !loaded.Excludes("nested/model.gen.go") || !loaded.Excludes("vendor/x/y.go") {
		t.Error("upgraded globs stopped matching")
	}

	rewritten, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rewritten), "generated_paths") || !strings.Contains(string(rewritten), "generatedPaths") {
		t.Errorf("the policy on disk was not rewritten:\n%s", rewritten)
	}
}

func TestLoadRefusesAPolicyFromAnUnknownGeneration(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".koment"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, FileName), []byte("apiVersion: koment.dev/v9\nkind: Policy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), APIVersion) {
		t.Fatalf("err = %v", err)
	}
}

func TestOnlyEnabledPrinciplesAreStated(t *testing.T) {
	configured := Default()
	if len(configured.States()) != 1 {
		t.Fatalf("the default policy states %d principles", len(configured.States()))
	}
	configured.Spec.Agents.Principles = nil
	if stated := configured.States(); len(stated) != 0 {
		t.Fatalf("a policy with no principles stated %v", stated)
	}
}
