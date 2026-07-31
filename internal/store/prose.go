package store

import "strings"

const ProseWidth = 76

const (
	SoftWrap       = "\n"
	ParagraphBreak = "\n\n"
)

// WrapProse hard-wraps body text at authoring time so that editing a sentence
// changes one line of a diff rather than all of it. It is deliberately not done
// when saving: the store round-trips whatever it is given, unchanged.
//
// A single newline is a soft wrap that a reader may re-flow; a blank line is a
// paragraph break that it must keep.
func WrapProse(text string) string {
	paragraphs := Paragraphs(text)
	wrapped := make([]string, len(paragraphs))
	for i, paragraph := range paragraphs {
		wrapped[i] = strings.Join(wrapParagraph(paragraph, ProseWidth), SoftWrap)
	}
	return strings.Join(wrapped, ParagraphBreak)
}

// Paragraphs splits body text into paragraphs, re-joining the soft wraps that
// WrapProse introduced so the text can be laid out again at another width.
func Paragraphs(text string) []string {
	var paragraphs []string
	for _, paragraph := range strings.Split(strings.TrimSpace(text), ParagraphBreak) {
		paragraphs = append(paragraphs, strings.Join(strings.Fields(paragraph), " "))
	}
	return paragraphs
}

func wrapParagraph(paragraph string, width int) []string {
	words := strings.Fields(paragraph)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	return append(lines, current)
}
