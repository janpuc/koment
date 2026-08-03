package cli

import (
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

func buildInfo(version string, settings ...debug.BuildSetting) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main:     debug.Module{Version: version},
			Settings: settings,
		}, true
	}
}

func TestAReleaseStampWins(t *testing.T) {
	got := describeBuild(
		Build{Version: "0.2.0", Revision: "0123456789abcdef0123"},
		buildInfo("v0.0.0-20260803042005-b8880cfaccc5"))

	if !strings.Contains(got, "koment 0.2.0") {
		t.Errorf("the stamped version should win over the module pseudo-version: %s", got)
	}
	if !strings.Contains(got, "0123456789ab") || strings.Contains(got, "0123456789abcdef0123") {
		t.Errorf("the revision should be abbreviated: %s", got)
	}
}

// go install records the module version, so a binary installed that way can
// report a real version without any linker stamping.
func TestAnInstalledBinaryReportsItsModuleVersion(t *testing.T) {
	got := describeBuild(Build{}, buildInfo("v0.1.2",
		debug.BuildSetting{Key: "vcs.revision", Value: "b8880cfaccc58dfcc23626df557a0fede86ddade"}))

	if !strings.Contains(got, "koment v0.1.2") {
		t.Errorf("want the module version, got %s", got)
	}
	if !strings.Contains(got, "b8880cfaccc5") {
		t.Errorf("want the recorded revision, got %s", got)
	}
}

func TestADirtyTreeSaysSo(t *testing.T) {
	got := describeBuild(Build{}, buildInfo("v0.1.2",
		debug.BuildSetting{Key: "vcs.revision", Value: "b8880cfaccc5"},
		debug.BuildSetting{Key: "vcs.modified", Value: "true"}))

	if !strings.Contains(got, "-dirty") {
		t.Errorf("a build from a modified tree must not claim to be its commit: %s", got)
	}
}

// Reporting a plausible-looking version for a build that has none would be the
// confident wrong answer this project exists to avoid.
func TestAnUnidentifiableBuildSaysUnknown(t *testing.T) {
	got := describeBuild(Build{}, func() (*debug.BuildInfo, bool) { return nil, false })

	if !strings.Contains(got, unknownVersion) {
		t.Errorf("want an admission that the version is unknown, got %s", got)
	}
	if !strings.Contains(got, runtime.GOOS+"/"+runtime.GOARCH) {
		t.Errorf("the platform should still be reported, got %s", got)
	}
}

func TestVersionExitsZeroAndPrintsToStdout(t *testing.T) {
	var stdout, stderr strings.Builder
	env := Environment{Stdout: &stdout, Stderr: &stderr, Build: Build{Version: "0.2.0"}}

	if code := Run([]string{"version"}, env, Servers{}); code != ExitOK {
		t.Fatalf("want exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "koment 0.2.0") {
		t.Errorf("version belongs on stdout, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
