package store

import (
	"strings"
	"testing"
	"time"
)

func TestNewIDHasCanonicalShapeAndAlphabet(t *testing.T) {
	id, err := NewID(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != ulidLength {
		t.Errorf("want %d characters, got %d in %q", ulidLength, len(id), id)
	}
	for _, c := range id {
		if !strings.ContainsRune(crockfordAlphabet, c) {
			t.Errorf("character %q in %q is outside the Crockford alphabet", c, id)
		}
	}
}

func TestNewIDSortsByCreationTime(t *testing.T) {
	earlier, err := NewID(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	later, err := NewID(time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if earlier >= later {
		t.Errorf("want %q < %q", earlier, later)
	}
}

func TestNewIDEncodesTheTimestampPrefix(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	first, err := NewID(at)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewID(at)
	if err != nil {
		t.Fatal(err)
	}

	const timestampChars = 10
	if first[:timestampChars] != second[:timestampChars] {
		t.Errorf("same instant produced different timestamp prefixes: %q and %q", first, second)
	}
	if first == second {
		t.Errorf("same instant produced identical ids, so the entropy is not random: %q", first)
	}
}

func TestNewIDIsUniqueUnderATightLoop(t *testing.T) {
	const count = 10000
	seen := make(map[string]struct{}, count)
	for range count {
		id, err := NewID(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if _, duplicate := seen[id]; duplicate {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestNewIDRejectsATimestampThatDoesNotFit(t *testing.T) {
	if _, err := NewID(time.Date(20000, 1, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("want an error for a timestamp beyond 48 bits, got nil")
	}
	if _, err := NewID(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("want an error for a timestamp before the epoch, got nil")
	}
}

func TestEncodeCrockfordCoversTheWholeValue(t *testing.T) {
	var lowest [timestampBytes + entropyBytes]byte
	if got, want := encodeCrockford(lowest), strings.Repeat("0", ulidLength); got != want {
		t.Errorf("want %q, got %q", want, got)
	}

	var highest [timestampBytes + entropyBytes]byte
	for i := range highest {
		highest[i] = 0xFF
	}
	want := "7" + strings.Repeat("Z", ulidLength-1)
	if got := encodeCrockford(highest); got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}
