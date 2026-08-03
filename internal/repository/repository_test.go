package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janpuc/koment/internal/store"
)

func withStore(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(root, store.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSingleRepositoryNeedsNoConfiguration(t *testing.T) {
	root := withStore(t, "myproject")

	set, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if set.Len() != 1 {
		t.Fatalf("want 1 repository, got %d", set.Len())
	}

	only, ok := set.Only()
	if !ok {
		t.Fatal("a single repository should resolve without being named")
	}
	if only.ID != "myproject" {
		t.Errorf("want the directory name as id, got %q", only.ID)
	}
	if only.Root != root {
		t.Errorf("want root %s, got %s", root, only.Root)
	}
}

func TestEnvListConfiguresSeveral(t *testing.T) {
	api := withStore(t, "api")
	web := withStore(t, "web")
	t.Setenv(EnvRepos, "api="+api+", web="+web)

	set, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if set.Len() != 2 {
		t.Fatalf("want 2 repositories, got %d", set.Len())
	}
	if _, ok := set.Only(); ok {
		t.Error("Only must not resolve when several are configured")
	}
	if got := set.IDs(); strings.Join(got, ",") != "api,web" {
		t.Errorf("want ordered ids api,web; got %v", got)
	}
}

func TestEnvListRejectsAMalformedEntry(t *testing.T) {
	t.Setenv(EnvRepos, "justapath")
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("want an error for an entry with no name=")
	}
}

func TestConfigFileCarriesTheRicherSettings(t *testing.T) {
	api := withStore(t, "api")
	config := filepath.Join(t.TempDir(), "koment.yaml")
	body := "repositories:\n" +
		"  - id: payments\n" +
		"    name: Payments API\n" +
		"    root: " + api + "\n" +
		"    clone_url: https://example.test/payments\n" +
		"    default_branch: trunk\n"
	if err := os.WriteFile(config, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfig, config)

	set, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := set.ByID("payments")
	if !ok {
		t.Fatalf("want the configured id, got %v", set.IDs())
	}
	if entry.Name != "Payments API" || entry.CloneURL != "https://example.test/payments" || entry.DefaultBranch != "trunk" {
		t.Errorf("settings did not survive: %+v", entry)
	}
	if entry.Display() != "Payments API" {
		t.Errorf("Display should prefer the name, got %q", entry.Display())
	}
}

func TestConfigFileRejectsUnknownFields(t *testing.T) {
	config := filepath.Join(t.TempDir(), "koment.yaml")
	body := "repositories:\n  - id: a\n    root: /tmp\n    colour: blue\n"
	if err := os.WriteFile(config, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfig, config)

	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("an unknown field should fail on load, not be ignored")
	}
}

func TestConfigFileResolvesRelativeRootsAgainstItself(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "api", store.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(base, "koment.yaml")
	if err := os.WriteFile(config, []byte("repositories:\n  - id: api\n    root: api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfig, config)

	set, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entry, _ := set.ByID("api")
	if entry.Root != filepath.Join(base, "api") {
		t.Errorf("a relative root should resolve against the config file, got %s", entry.Root)
	}
}

func TestDuplicateIDsAreRefused(t *testing.T) {
	one := withStore(t, "one")
	two := withStore(t, "two")
	t.Setenv(EnvRepos, "same="+one+",same="+two)

	if _, err := Load(t.TempDir()); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want a duplicate id error, got %v", err)
	}
}

func TestIdentityIsAssignedNotDerivedFromThePath(t *testing.T) {
	first := withStore(t, "api")
	second := withStore(t, "api-moved")
	t.Setenv(EnvRepos, "api="+first)

	before, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv(EnvRepos, "api="+second)
	after, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	from, _ := before.Only()
	to, _ := after.Only()
	if from.ID != to.ID {
		t.Errorf("identity must survive a move: %s became %s", from.ID, to.ID)
	}
	if from.Root == to.Root {
		t.Fatal("the test did not actually move the repository")
	}
}

func TestResolveAcceptsAnIDOrADisplayName(t *testing.T) {
	api := withStore(t, "api")
	config := filepath.Join(t.TempDir(), "koment.yaml")
	body := "repositories:\n  - id: payments\n    name: Payments API\n    root: " + api + "\n"
	if err := os.WriteFile(config, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfig, config)

	set, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, reference := range []string{"payments", "Payments API", "payments api"} {
		if _, ok := set.Resolve(reference); !ok {
			t.Errorf("Resolve(%q) should find the repository", reference)
		}
	}
	if _, ok := set.Resolve("nothing"); ok {
		t.Error("Resolve should not invent a repository")
	}
}

func TestConfigBeatsListBeatsSingle(t *testing.T) {
	fromConfig := withStore(t, "fromconfig")
	fromList := withStore(t, "fromlist")
	single := withStore(t, "single")

	config := filepath.Join(t.TempDir(), "koment.yaml")
	if err := os.WriteFile(config, []byte("repositories:\n  - id: winner\n    root: "+fromConfig+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfig, config)
	t.Setenv(EnvRepos, "list="+fromList)
	t.Setenv(EnvRepo, single)

	set, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ids := set.IDs(); len(ids) != 1 || ids[0] != "winner" {
		t.Errorf("the config file should win, got %v", ids)
	}
}
