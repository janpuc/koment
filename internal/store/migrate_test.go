package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const legacyRecordYAML = `# yaml-language-server: $schema=` + SchemaURL + `
version: 1
id: ` + firstID + `
file: internal/session/token.go
kind: invariant
title: The prior key survives the rotation window
body: Keep the prior key through the rotation window.
created: "2026-08-03"
anchor:
  scope: excerpt
  excerpt: "if token.Expired(now) {"
  before: "func rotate() {"
  last_seen_line: 42
author:
  name: Fixture Author
  kind: human
  source: explicit
`

func writeRecordFile(t *testing.T, annotations *Store, id, content string) string {
	t.Helper()
	path, err := annotations.RecordPath(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadUpgradesAVersionOneRecordAndRewritesItInPlace(t *testing.T) {
	annotations := newTestStore(t)
	path := writeRecordFile(t, annotations, firstID, legacyRecordYAML)

	loaded, err := annotations.Load(firstID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.APIVersion != APIVersion || loaded.Kind != KindAnnotation {
		t.Fatalf("record was not upgraded: %+v", loaded)
	}
	if loaded.Spec.Target.File != "internal/session/token.go" || loaded.Spec.Type != TypeInvariant {
		t.Errorf("spec did not carry the v1 fields: %+v", loaded.Spec)
	}
	if loaded.Status.LastSeenLine != 42 {
		t.Errorf("last_seen_line became status.lastSeenLine as %d", loaded.Status.LastSeenLine)
	}
	if loaded.Status.Resolution != "" {
		t.Errorf("upgrading is not resolving, but a resolution was recorded: %q", loaded.Status.Resolution)
	}
	if want := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC); !loaded.Metadata.Created.Equal(want) {
		t.Errorf("created = %s, want %s", loaded.Metadata.Created, want)
	}

	rewritten := readFile(t, path)
	if strings.Contains(rewritten, "version: 1") || !strings.Contains(rewritten, "apiVersion: "+APIVersion) {
		t.Fatalf("the record on disk was not rewritten:\n%s", rewritten)
	}
}

func TestUpgradingARecordTwiceChangesNothingTheSecondTime(t *testing.T) {
	annotations := newTestStore(t)
	path := writeRecordFile(t, annotations, firstID, legacyRecordYAML)

	if _, err := annotations.Load(firstID); err != nil {
		t.Fatal(err)
	}
	upgraded := readFile(t, path)
	if _, err := annotations.Load(firstID); err != nil {
		t.Fatal(err)
	}
	if again := readFile(t, path); again != upgraded {
		t.Errorf("a second read changed the record\n first:\n%s\nsecond:\n%s", upgraded, again)
	}
}

func TestLoadRefusesARecordFromAnUnknownGeneration(t *testing.T) {
	annotations := newTestStore(t)
	for name, content := range map[string]string{
		"a later api version": strings.Replace(
			strings.Replace(legacyRecordYAML, "version: 1", "apiVersion: koment.dev/v9", 1),
			"kind: invariant", "kind: Annotation", 1),
		"no version marker at all": strings.Replace(legacyRecordYAML, "version: 1\n", "", 1),
		"a version koment never wrote": strings.Replace(
			legacyRecordYAML, "version: 1", "version: 2", 1),
	} {
		t.Run(name, func(t *testing.T) {
			writeRecordFile(t, annotations, firstID, content)
			_, err := annotations.Load(firstID)
			if err == nil {
				t.Fatal("an unreadable generation was accepted")
			}
			if !strings.Contains(err.Error(), APIVersion) {
				t.Errorf("the error does not say which version this binary reads: %v", err)
			}
		})
	}
}

func TestObserveKeepsTheTimeAnObservationFirstBecameTrue(t *testing.T) {
	commit := strings.Repeat("a", 40)
	first := Timestamp{time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)}
	later := Timestamp{time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)}

	status := Status{}
	status.Observe(AnchorOK, commit, first)
	status.Observe(AnchorOK, commit, later)
	if !status.ResolvedAt.Equal(first.Time) {
		t.Errorf("an unchanged observation moved resolvedAt to %s", status.ResolvedAt)
	}

	status.Observe(AnchorDrifted, commit, later)
	if !status.ResolvedAt.Equal(later.Time) || status.Resolution != AnchorDrifted {
		t.Errorf("a changed observation was not recorded: %+v", status)
	}
}

func TestTimestampReadsBothGenerationsQuotedOrNot(t *testing.T) {
	for _, written := range []string{
		`created: "2026-08-03"`,
		"created: 2026-08-03",
		`created: "2026-08-03T09:15:00Z"`,
		"created: 2026-08-03T09:15:00Z",
	} {
		t.Run(written, func(t *testing.T) {
			var holder struct {
				Created Timestamp `yaml:"created"`
			}
			if err := decodeOneDocument("fixture", []byte(written+"\n"), &holder); err != nil {
				t.Fatalf("decode %q: %v", written, err)
			}
			if holder.Created.IsZero() || holder.Created.Year() != 2026 {
				t.Errorf("%q decoded to %s", written, holder.Created)
			}
		})
	}
}
