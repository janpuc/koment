package commentpolicy

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"unicode"

	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/policy"
	"github.com/janpuc/koment/internal/store"
)

// SourceComment is an exact syntactic comment group in one source file.
type SourceComment struct {
	File  string
	Raw   string
	Body  string
	Line  int
	Start int
	End   int
}

// Violation is a prohibited comment and the action that resolves it.
type Violation struct {
	Comment SourceComment
	Reason  string
}

// Conversion is the source edit and deterministic code anchor for comment intent.
type Conversion struct {
	Content []byte
	Body    string
	Excerpt string
}

// IsCommentIntent distinguishes prose from text that parses as Go statements.
func IsCommentIntent(comment SourceComment) bool {
	body := strings.TrimSpace(comment.Body)
	if body == "" {
		return false
	}
	candidate := "package intent\nfunc inspect() {\n" + body + "\n}\n"
	_, err := parser.ParseFile(token.NewFileSet(), comment.File, candidate, parser.SkipObjectResolution)
	return err != nil
}

// Check scans supported source and returns every prohibited comment.
func Check(rootPath string, configured policy.Policy, requested []string) (_ []Violation, returnedError error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("opening repository root %s: %w", rootPath, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			returnedError = errors.Join(returnedError, closeErr)
		}
	}()

	files, err := sourceFiles(root, configured, requested)
	if err != nil {
		return nil, err
	}
	records, err := store.Open(rootPath).All()
	if err != nil {
		return nil, err
	}

	var violations []Violation
	for _, file := range files {
		content, err := root.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", file, err)
		}
		found, err := CheckContent(file, content, configured, records)
		if err != nil {
			return nil, err
		}
		violations = append(violations, found...)
	}
	return violations, nil
}

// CheckContent applies comment policy to an in-memory source document.
func CheckContent(file string, content []byte, configured policy.Policy, records []store.Annotation) ([]Violation, error) {
	comments, intrinsic, err := scan(file, content, configured)
	if err != nil {
		return nil, err
	}
	violations := make([]Violation, 0, len(comments))
	for index, comment := range comments {
		if intrinsic[index] || acknowledged(comment, content, records) {
			continue
		}
		violations = append(violations, Violation{
			Comment: comment,
			Reason:  "convert with `koment comments convert` or retain with `koment comments acknowledge --acknowledge-inline-comment`",
		})
	}
	return violations, nil
}

// Find returns the one comment group matching a verbatim excerpt.
func Find(file string, content []byte, excerpt string) (SourceComment, error) {
	comments, _, err := scan(file, content, policy.Default())
	if err != nil {
		return SourceComment{}, err
	}
	var matches []SourceComment
	for _, comment := range comments {
		if comment.Raw == excerpt || strings.TrimSpace(comment.Raw) == strings.TrimSpace(excerpt) {
			matches = append(matches, comment)
		}
	}
	switch len(matches) {
	case 0:
		return SourceComment{}, fmt.Errorf("comment excerpt does not match a complete syntactic comment group in %s", file)
	case 1:
		return matches[0], nil
	default:
		return SourceComment{}, fmt.Errorf("comment excerpt matches %d groups in %s; include the complete distinct group", len(matches), file)
	}
}

// Convert removes one comment and selects nearby code as its annotation anchor.
func Convert(content []byte, comment SourceComment) (Conversion, error) {
	converted, removedAt := remove(content, comment)
	if _, err := parser.ParseFile(token.NewFileSet(), comment.File, converted, parser.SkipObjectResolution); err != nil {
		return Conversion{}, fmt.Errorf("removing the comment would leave invalid Go source: %w", err)
	}
	excerpt, err := codeAnchor(converted, content, removedAt)
	if err != nil {
		return Conversion{}, err
	}
	return Conversion{Content: converted, Body: comment.Body, Excerpt: excerpt}, nil
}

// AcknowledgementExcerpt selects a unique anchor that contains the retained comment.
func AcknowledgementExcerpt(content []byte, comment SourceComment) (string, error) {
	if _, err := anchor.Capture(content, comment.Raw); err == nil {
		return comment.Raw, nil
	}
	starts := lineStarts(content)
	startLine := lineOf(starts, comment.Start)
	endLine := lineOf(starts, comment.End-1)
	for radius := 0; radius <= 5; radius++ {
		first := max(0, startLine-radius)
		last := min(len(starts)-1, endLine+radius)
		excerpt := strings.TrimSpace(string(content[starts[first]:lineEnd(content, starts, last)]))
		if strings.Contains(excerpt, comment.Raw) {
			if _, err := anchor.Capture(content, excerpt); err == nil {
				return excerpt, nil
			}
		}
	}
	return "", fmt.Errorf("the retained comment cannot be anchored uniquely; make the comment or surrounding code more specific")
}

// Detector finds the comment groups one language exposes and says which of them
// its toolchain requires. koment ships one, and the interface exists so a
// second language is an implementation rather than a rewrite (ADR 0114).
type Detector interface {
	Handles(file string) bool
	Scan(file string, content []byte, configured policy.Policy) ([]SourceComment, []bool, error)
}

var detectors = []Detector{goDetector{}}

func detectorFor(file string) Detector {
	for _, candidate := range detectors {
		if candidate.Handles(file) {
			return candidate
		}
	}
	return nil
}

// Detects reports whether any language koment understands claims this file.
func Detects(file string) bool {
	return detectorFor(file) != nil
}

func scan(file string, content []byte, configured policy.Policy) ([]SourceComment, []bool, error) {
	detector := detectorFor(file)
	if detector == nil {
		return nil, nil, fmt.Errorf("no comment detector for %s", file)
	}
	return detector.Scan(file, content, configured)
}

type goDetector struct{}

func (goDetector) Handles(file string) bool { return strings.HasSuffix(file, ".go") }

func (goDetector) Scan(file string, content []byte, configured policy.Policy) ([]SourceComment, []bool, error) {
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, file, content, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", file, err)
	}
	public := publicDocumentation(parsed)
	comments := make([]SourceComment, 0, len(parsed.Comments))
	intrinsic := make([]bool, 0, len(parsed.Comments))
	for _, group := range parsed.Comments {
		start := files.Position(group.Pos())
		end := files.Position(group.End())
		raw := string(content[start.Offset:end.Offset])
		comments = append(comments, SourceComment{
			File: file, Raw: raw, Body: commentBody(raw), Line: start.Line,
			Start: start.Offset, End: end.Offset,
		})
		intrinsic = append(intrinsic, isIntrinsic(group, raw, public[group], configured))
	}
	return comments, intrinsic, nil
}

func publicDocumentation(file *ast.File) map[*ast.CommentGroup]bool {
	public := map[*ast.CommentGroup]bool{}
	mark := func(group *ast.CommentGroup) {
		if group != nil {
			public[group] = true
		}
	}
	mark(file.Doc)
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.FuncDecl:
			if ast.IsExported(value.Name.Name) {
				mark(value.Doc)
			}
		case *ast.GenDecl:
			if exportedSpecs(value.Specs) {
				mark(value.Doc)
			}
			for _, specification := range value.Specs {
				switch declared := specification.(type) {
				case *ast.TypeSpec:
					if ast.IsExported(declared.Name.Name) {
						mark(declared.Doc)
						mark(declared.Comment)
					}
				case *ast.ValueSpec:
					if exportedNames(declared.Names) {
						mark(declared.Doc)
						mark(declared.Comment)
					}
				}
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		field, ok := node.(*ast.Field)
		if ok && exportedNames(field.Names) {
			mark(field.Doc)
			mark(field.Comment)
		}
		return true
	})
	return public
}

func exportedSpecs(specifications []ast.Spec) bool {
	for _, specification := range specifications {
		switch value := specification.(type) {
		case *ast.TypeSpec:
			if ast.IsExported(value.Name.Name) {
				return true
			}
		case *ast.ValueSpec:
			if exportedNames(value.Names) {
				return true
			}
		}
	}
	return false
}

func exportedNames(names []*ast.Ident) bool {
	for _, name := range names {
		if ast.IsExported(name.Name) {
			return true
		}
	}
	return false
}

func isIntrinsic(group *ast.CommentGroup, raw string, public bool, configured policy.Policy) bool {
	switch {
	case public && configured.Allows(policy.IntrinsicPublicAPI):
		return true
	case configured.Allows(policy.IntrinsicDeprecated) && strings.Contains(group.Text(), "Deprecated:"):
		return true
	case configured.Allows(policy.IntrinsicUpstreamLink) &&
		(strings.Contains(raw, "https://") || strings.Contains(raw, "http://")):
		return true
	case configured.Allows(policy.IntrinsicGeneratedMarker) &&
		strings.Contains(raw, "Code generated") && strings.Contains(raw, "DO NOT EDIT."):
		return true
	case configured.Allows(policy.IntrinsicToolchain) && directivesOnly(group):
		return true
	default:
		return false
	}
}

func directivesOnly(group *ast.CommentGroup) bool {
	for _, comment := range group.List {
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(comment.Text, "//"), "/*"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))
		if !hasDirectivePrefix(text) {
			return false
		}
	}
	return len(group.List) > 0
}

func hasDirectivePrefix(text string) bool {
	prefixes := []string{"go:", "+build", "line ", "nolint", "lint:", "revive:", "gosec", "export ", "#cgo"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func acknowledged(comment SourceComment, content []byte, records []store.Annotation) bool {
	for _, record := range records {
		if record.File != comment.File || record.Policy == nil ||
			record.Policy.Exception != "inline-comment" || !record.Policy.Acknowledged ||
			!strings.Contains(record.Anchor.Excerpt, comment.Raw) {
			continue
		}
		if !anchor.Resolve(record, content).Status.IsFailure() {
			return true
		}
	}
	return false
}

func sourceFiles(root *os.Root, configured policy.Policy, requested []string) ([]string, error) {
	starts := requested
	if len(starts) == 0 {
		starts = []string{"."}
	}
	seen := map[string]bool{}
	for _, requestedPath := range starts {
		clean, err := sourcePath(requestedPath)
		if err != nil {
			return nil, err
		}
		if err := fs.WalkDir(root.FS(), clean, func(file string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() && (file == ".git" || file == ".koment" || configured.Excludes(file+"/placeholder")) {
				return fs.SkipDir
			}
			if !entry.IsDir() && Detects(file) && !configured.Excludes(file) {
				seen[file] = true
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("walking %s: %w", clean, err)
		}
	}
	files := make([]string, 0, len(seen))
	for file := range seen {
		files = append(files, file)
	}
	sort.Strings(files)
	return files, nil
}

func sourcePath(value string) (string, error) {
	if strings.Contains(value, `\`) || strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("source path %s must be repository-relative and use forward slashes", value)
	}
	clean := path.Clean(value)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("source path %s escapes the repository", value)
	}
	return clean, nil
}

func commentBody(raw string) string {
	lines := strings.Split(raw, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimSpace(strings.TrimPrefix(line, "//"))
		line = strings.TrimSpace(strings.TrimPrefix(line, "/*"))
		line = strings.TrimSpace(strings.TrimSuffix(line, "*/"))
		line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func remove(content []byte, comment SourceComment) ([]byte, int) {
	start, end := comment.Start, comment.End
	lineStart := bytes.LastIndexByte(content[:start], '\n') + 1
	lineEnd := len(content)
	if next := bytes.IndexByte(content[end:], '\n'); next >= 0 {
		lineEnd = end + next + 1
	}
	beforeOnlySpace := len(bytes.TrimSpace(content[lineStart:start])) == 0
	afterEnd := lineEnd
	if afterEnd > end && content[afterEnd-1] == '\n' {
		afterEnd--
	}
	afterOnlySpace := len(bytes.TrimSpace(content[end:afterEnd])) == 0
	if beforeOnlySpace && afterOnlySpace {
		converted := append([]byte{}, content[:lineStart]...)
		converted = append(converted, content[lineEnd:]...)
		return converted, lineStart
	}
	for start > lineStart && (content[start-1] == ' ' || content[start-1] == '\t') {
		start--
	}
	converted := append([]byte{}, content[:start]...)
	if start > 0 && end < len(content) && !unicode.IsSpace(rune(content[start-1])) && !unicode.IsSpace(rune(content[end])) {
		converted = append(converted, ' ')
	}
	converted = append(converted, content[end:]...)
	return converted, start
}

func codeAnchor(content, original []byte, near int) (string, error) {
	starts := lineStarts(content)
	line := lineOf(starts, min(near, max(0, len(content)-1)))
	for distance := 0; distance < len(starts); distance++ {
		candidates := []int{line + distance}
		if distance > 0 {
			candidates = append(candidates, line-distance)
		}
		for _, candidate := range candidates {
			if candidate < 0 || candidate >= len(starts) {
				continue
			}
			for radius := 0; radius <= 5 && candidate+radius < len(starts); radius++ {
				excerpt := strings.TrimSpace(string(content[starts[candidate]:lineEnd(content, starts, candidate+radius)]))
				if excerpt == "" || strings.HasPrefix(excerpt, "//") || strings.HasPrefix(excerpt, "/*") {
					continue
				}
				if _, err := anchor.Capture(content, excerpt); err == nil {
					if _, originalErr := anchor.Capture(original, excerpt); originalErr == nil {
						return excerpt, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("cannot select a unique code anchor after removing the comment")
}

func lineStarts(content []byte) []int {
	starts := []int{0}
	for index, character := range content {
		if character == '\n' && index+1 < len(content) {
			starts = append(starts, index+1)
		}
	}
	return starts
}

func lineOf(starts []int, offset int) int {
	index := sort.Search(len(starts), func(index int) bool { return starts[index] > offset })
	return max(0, index-1)
}

func lineEnd(content []byte, starts []int, line int) int {
	if line+1 < len(starts) {
		return starts[line+1] - 1
	}
	return len(content)
}
