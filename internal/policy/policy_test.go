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
	configured.Comments.GeneratedPaths = append(configured.Comments.GeneratedPaths, "generated/**")
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
	if err != nil || created || second.Version != Version {
		t.Fatalf("second install = %#v, %v, %v", second, created, err)
	}
}

func TestPolicyRejectsBypassesAndUnknownValues(t *testing.T) {
	cases := map[string]func(*Policy){
		"non-strict mode":   func(value *Policy) { value.Comments.Mode = "off" },
		"escaping pattern":  func(value *Policy) { value.Comments.VendoredPaths = []string{"../vendor/**"} },
		"absolute pattern":  func(value *Policy) { value.Comments.VendoredPaths = []string{"/vendor/**"} },
		"unknown intrinsic": func(value *Policy) { value.Comments.Intrinsic = []Intrinsic{"anything"} },
		"unknown adapter":   func(value *Policy) { value.Agents.Adapters = []Adapter{"anything"} },
		"duplicate adapter": func(value *Policy) { value.Agents.Adapters = []Adapter{AdapterAgents, AdapterAgents} },
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
