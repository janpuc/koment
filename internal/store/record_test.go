package store

import (
	"strings"
	"testing"
)

func TestHeadlinePrefersATitleAndFallsBackToTheFirstSentence(t *testing.T) {
	for _, test := range []struct {
		name  string
		title string
		body  string
		want  string
	}{
		{"a title wins", "Rotation keeps the previous key", "Anything at all. Second sentence.", "Rotation keeps the previous key"},
		{"first sentence", "", "Skew is deliberate. Clients whose clock runs fast get logged out.", "Skew is deliberate"},
		{"one sentence body", "", "The upstream closes idle connections", "The upstream closes idle connections"},
		{"newlines collapse", "", "wrapped across\ntwo lines. Next.", "wrapped across two lines"},
		{"a decimal is not a full stop", "", "Pinned to 1.2.3 because arm64 broke", "Pinned to 1.2.3 because arm64 broke"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := Annotation{
				APIVersion: APIVersion,
				Kind:       KindAnnotation,
				Spec: Spec{
					Title: test.title,
					Body:  test.body,
				},
			}.Headline()
			if got != test.want {
				t.Errorf("Headline() = %q, want %q", got, test.want)
			}
		})
	}
}

// The headline is what renders beside code without being shortened, so it can
// never itself exceed the limit.
func TestADerivedHeadlineNeverExceedsTheLimit(t *testing.T) {
	long := strings.Repeat("consideration ", 40)
	got := Annotation{
		APIVersion: APIVersion,
		Kind:       KindAnnotation,
		Spec: Spec{
			Body: long,
		},
	}.Headline()
	if count := len([]rune(got)); count > TitleLimit {
		t.Errorf("derived headline is %d characters, limit is %d: %q", count, TitleLimit, got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("a shortened headline should say it was shortened: %q", got)
	}
	if strings.Contains(got, "considerati…") {
		t.Errorf("shortened mid-word rather than at a space: %q", got)
	}
}

func TestATitleIsOneShortLine(t *testing.T) {
	for _, test := range []struct {
		name  string
		title string
		valid bool
	}{
		{"absent", "", true},
		{"ordinary", "Rotation keeps the previous key", true},
		{"blank", "   ", false},
		{"newline", "two\nlines", false},
		{"carriage return", "two\rlines", false},
		{"at the limit", strings.Repeat("a", TitleLimit), true},
		{"over the limit", strings.Repeat("a", TitleLimit+1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validTitle("01JQ8ZK3M4N5P6R7S8T9V0W1X2", test.title)
			if test.valid && err != nil {
				t.Errorf("want valid, got %v", err)
			}
			if !test.valid && err == nil {
				t.Error("want an error")
			}
		})
	}
}
