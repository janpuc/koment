package config

import (
	"flag"
	"io"
	"strings"
	"testing"
)

func flagSet() (*flag.FlagSet, *string, *int) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	listen := flags.String("listen", "default", "")
	port := flags.Int("metrics-port", 0, "")
	return flags, listen, port
}

func TestEnvNameMatchesTheFlag(t *testing.T) {
	cases := map[string]string{
		"listen":          "KOMENT_LISTEN",
		"streamable-http": "KOMENT_STREAMABLE_HTTP",
		"out":             "KOMENT_OUT",
	}
	for flagName, want := range cases {
		if got := EnvName(flagName); got != want {
			t.Errorf("EnvName(%q) = %q, want %q", flagName, got, want)
		}
	}
}

func TestEnvironmentFillsInAFlagThatWasNotGiven(t *testing.T) {
	t.Setenv("KOMENT_LISTEN", "0.0.0.0:9999")
	flags, listen, _ := flagSet()

	if err := flags.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if err := FromEnvironment(flags); err != nil {
		t.Fatal(err)
	}
	if *listen != "0.0.0.0:9999" {
		t.Errorf("want the environment value, got %q", *listen)
	}
}

func TestAnExplicitFlagBeatsTheEnvironment(t *testing.T) {
	t.Setenv("KOMENT_LISTEN", "0.0.0.0:9999")
	flags, listen, _ := flagSet()

	if err := flags.Parse([]string{"--listen", "127.0.0.1:1234"}); err != nil {
		t.Fatal(err)
	}
	if err := FromEnvironment(flags); err != nil {
		t.Fatal(err)
	}
	if *listen != "127.0.0.1:1234" {
		t.Errorf("an explicit flag must win, got %q", *listen)
	}
}

func TestAnUnsetVariableLeavesTheDefault(t *testing.T) {
	flags, listen, _ := flagSet()
	if err := flags.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if err := FromEnvironment(flags); err != nil {
		t.Fatal(err)
	}
	if *listen != "default" {
		t.Errorf("want the flag default, got %q", *listen)
	}
}

func TestAnUnparseableValueFailsLoudly(t *testing.T) {
	t.Setenv("KOMENT_METRICS_PORT", "not-a-number")
	flags, _, _ := flagSet()
	if err := flags.Parse(nil); err != nil {
		t.Fatal(err)
	}

	err := FromEnvironment(flags)
	if err == nil {
		t.Fatal("a bad environment value must not be silently ignored")
	}
	for _, want := range []string{"KOMENT_METRICS_PORT", "not-a-number", "metrics-port"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should name %q, got: %v", want, err)
		}
	}
}

func TestUsageDocumentsBothWaysOfSettingAFlag(t *testing.T) {
	flags, _, _ := flagSet()

	usage := Usage(flags)
	for _, want := range []string{"--listen", "KOMENT_LISTEN", "--metrics-port", "KOMENT_METRICS_PORT"} {
		if !strings.Contains(usage, want) {
			t.Errorf("usage is missing %q:\n%s", want, usage)
		}
	}
}

func TestRootReportsWhetherItWasSet(t *testing.T) {
	if _, ok := Root(); ok {
		t.Error("KOMENT_REPO is not set in this test")
	}

	t.Setenv("KOMENT_REPO", "/repo")
	root, ok := Root()
	if !ok || root != "/repo" {
		t.Errorf("want /repo, got %q (set=%v)", root, ok)
	}

	t.Setenv("KOMENT_REPO", "")
	if _, ok := Root(); ok {
		t.Error("an empty KOMENT_REPO means unset, not a repository at the filesystem root")
	}
}
