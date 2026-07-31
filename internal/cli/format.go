package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/store"
)

const bodyWidth = 74

func writeResolution(w io.Writer, file string, resolution anchor.Resolution) {
	fmt.Fprintf(w, "  %-9s %-13s %s  %s\n",
		resolution.Status,
		resolution.Annotation.Kind,
		location(file, resolution.Line),
		resolution.Annotation.ID)

	if resolution.Occurrences > 1 {
		fmt.Fprintf(w, "    (excerpt matches %d places in this file)\n", resolution.Occurrences)
	}
	for _, line := range wrap(resolution.Annotation.Body, bodyWidth) {
		fmt.Fprintf(w, "    %s\n", line)
	}
}

func location(file string, line int) string {
	if line == 0 {
		return file
	}
	return fmt.Sprintf("%s:%d", file, line)
}

func wrap(body string, width int) []string {
	var lines []string
	for i, paragraph := range store.Paragraphs(body) {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, wrapParagraph(paragraph, width)...)
	}
	return lines
}

func wrapParagraph(paragraph string, width int) []string {
	if paragraph == "" {
		return []string{""}
	}

	var lines []string
	current := ""
	for _, word := range strings.Fields(paragraph) {
		switch {
		case current == "":
			current = word
		case len(current)+1+len(word) <= width:
			current += " " + word
		default:
			lines = append(lines, current)
			current = word
		}
	}
	return append(lines, current)
}
