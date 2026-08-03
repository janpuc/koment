package cli

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Build is what a release stamps in at link time. It is empty for a build made
// any other way, and that is the normal case rather than a fault: Go records
// the module version and the revision in the binary itself, so describeBuild
// falls back to those instead of inventing a number.
type Build struct {
	Version  string
	Revision string
}

const unknownVersion = "unknown"

func runVersion(_ []string, env Environment) int {
	fmt.Fprintln(env.Stdout, describeBuild(env.Build, debug.ReadBuildInfo))
	return ExitOK
}

func describeBuild(stamped Build, read func() (*debug.BuildInfo, bool)) string {
	version, revision := stamped.Version, stamped.Revision

	if info, ok := read(); ok {
		if version == "" {
			version = info.Main.Version
		}
		if revision == "" {
			revision = settingOf(info, "vcs.revision")
		}
		if settingOf(info, "vcs.modified") == "true" {
			revision += "-dirty"
		}
	}

	if version == "" {
		version = unknownVersion
	}
	described := "koment " + version
	if revision != "" {
		described += " (" + abbreviate(revision) + ")"
	}
	return described + " " + runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH
}

func settingOf(info *debug.BuildInfo, key string) string {
	for _, setting := range info.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}

func abbreviate(revision string) string {
	hash, suffix, _ := strings.Cut(revision, "-")
	if len(hash) > 12 {
		hash = hash[:12]
	}
	if suffix != "" {
		return hash + "-" + suffix
	}
	return hash
}
