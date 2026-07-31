package listen

import (
	"strings"
	"testing"
)

func TestAddressDefaultsToLoopback(t *testing.T) {
	cases := map[string]string{
		"8765":            "127.0.0.1:8765",
		":8765":           "127.0.0.1:8765",
		"127.0.0.1:8765":  "127.0.0.1:8765",
		"0.0.0.0:8765":    "0.0.0.0:8765",
		"localhost:8765":  "localhost:8765",
		"192.168.1.5:900": "192.168.1.5:900",
	}

	for address, want := range cases {
		got, err := Address(address)
		if err != nil {
			t.Errorf("Address(%q): %v", address, err)
			continue
		}
		if got != want {
			t.Errorf("Address(%q) = %q, want %q", address, got, want)
		}
	}
}

func TestAddressRejectsNonsense(t *testing.T) {
	for _, address := range []string{"not-a-port", "1:2:3", ""} {
		if got, err := Address(address); err == nil {
			t.Errorf("Address(%q) = %q, want an error", address, got)
		}
	}
}

func TestWarnIfPublicNamesTheConsequence(t *testing.T) {
	var warning strings.Builder
	WarnIfPublic("0.0.0.0:8765", &warning)

	if !strings.Contains(warning.String(), "WARNING") {
		t.Errorf("binding to all interfaces must warn, got %q", warning.String())
	}
	if !strings.Contains(warning.String(), "no authentication") {
		t.Errorf("the warning must say why it matters, got %q", warning.String())
	}
}

func TestWarnIfPublicStaysQuietOnLoopback(t *testing.T) {
	for _, address := range []string{"127.0.0.1:8765", "localhost:8765", "[::1]:8765"} {
		var quiet strings.Builder
		WarnIfPublic(address, &quiet)
		if quiet.String() != "" {
			t.Errorf("%s is local, must not warn, got %q", address, quiet.String())
		}
	}
}
