// Package config lets every flag be set from the environment, which is how a
// container is configured.
package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const Prefix = "KOMENT_"

// EnvName is the variable that stands in for a flag: --streamable-http becomes
// KOMENT_STREAMABLE_HTTP.
func EnvName(flagName string) string {
	return Prefix + strings.ToUpper(strings.ReplaceAll(flagName, "-", "_"))
}

// FromEnvironment fills in any flag the caller did not pass. An explicit flag
// always wins, so a container's environment sets the default and a person
// debugging it can still override on the command line.
//
// It runs after Parse because that is the only point at which "was this flag
// actually given" is knowable.
func FromEnvironment(flags *flag.FlagSet) error {
	given := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { given[f.Name] = true })

	var failed error
	flags.VisitAll(func(f *flag.Flag) {
		if given[f.Name] || failed != nil {
			return
		}
		value, ok := os.LookupEnv(EnvName(f.Name))
		if !ok {
			return
		}
		if err := f.Value.Set(value); err != nil {
			failed = fmt.Errorf("%s=%q is not valid for --%s: %w", EnvName(f.Name), value, f.Name, err)
		}
	})
	return failed
}

// Usage renders the environment variable for each flag, so --help documents
// both ways of setting it rather than only one.
func Usage(flags *flag.FlagSet) string {
	var out strings.Builder
	flags.VisitAll(func(f *flag.Flag) {
		fmt.Fprintf(&out, "  --%-18s %-24s %s\n", f.Name, EnvName(f.Name), f.Usage)
	})
	return out.String()
}

// Root is the repository koment serves. It exists for containers, where the
// working directory is a mount point rather than somewhere a person cd'd to.
func Root() (string, bool) {
	root, ok := os.LookupEnv(Prefix + "REPO")
	return root, ok && root != ""
}
