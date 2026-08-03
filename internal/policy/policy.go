package policy

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

const (
	Version    = 1
	ModeStrict = "strict"
	FileName   = ".koment/policy.yaml"
	SchemaURL  = "https://raw.githubusercontent.com/janpuc/koment/main/schema/policy.schema.json"
)

// Intrinsic names one class of source comment that may remain inline.
type Intrinsic string

const (
	IntrinsicToolchain       Intrinsic = "toolchain-directive"
	IntrinsicGeneratedMarker Intrinsic = "generated-marker"
	IntrinsicUpstreamLink    Intrinsic = "upstream-link"
	IntrinsicDeprecated      Intrinsic = "deprecated"
	IntrinsicPublicAPI       Intrinsic = "public-api"
)

// Adapter names one generated agent instruction surface.
type Adapter string

const (
	AdapterAgents  Adapter = "agents"
	AdapterClaude  Adapter = "claude"
	AdapterCopilot Adapter = "copilot"
	AdapterCursor  Adapter = "cursor"
	AdapterCodex   Adapter = "codex"
)

// Policy is the version-1 repository enforcement contract.
type Policy struct {
	Version  int            `yaml:"version"`
	Comments CommentsPolicy `yaml:"comments"`
	Agents   AgentsPolicy   `yaml:"agents"`
}

// CommentsPolicy configures strict classification and repository exclusions.
type CommentsPolicy struct {
	Mode           string      `yaml:"mode"`
	Intrinsic      []Intrinsic `yaml:"intrinsic"`
	GeneratedPaths []string    `yaml:"generated_paths,omitempty"`
	VendoredPaths  []string    `yaml:"vendored_paths,omitempty"`
}

// AgentsPolicy selects generated instruction adapters.
type AgentsPolicy struct {
	Adapters []Adapter `yaml:"adapters"`
}

// Default returns the strict policy installed for a new repository.
func Default() Policy {
	return Policy{
		Version: Version,
		Comments: CommentsPolicy{
			Mode: ModeStrict,
			Intrinsic: []Intrinsic{
				IntrinsicToolchain, IntrinsicGeneratedMarker, IntrinsicUpstreamLink,
				IntrinsicDeprecated, IntrinsicPublicAPI,
			},
			GeneratedPaths: []string{"**/*.gen.go", "**/*.generated.go"},
			VendoredPaths:  []string{"vendor/**"},
		},
		Agents: AgentsPolicy{Adapters: []Adapter{
			AdapterAgents, AdapterClaude, AdapterCopilot, AdapterCursor, AdapterCodex,
		}},
	}
}

// Load reads and strictly validates the repository policy.
func Load(rootPath string) (configured Policy, returnedError error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return Policy{}, fmt.Errorf("opening repository root %s: %w", rootPath, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			returnedError = errors.Join(returnedError, closeErr)
		}
	}()
	content, err := root.ReadFile(FileName)
	if err != nil {
		return Policy{}, fmt.Errorf("reading %s: %w", FileName, err)
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&configured); err != nil {
		return Policy{}, fmt.Errorf("parsing %s: %w", FileName, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Policy{}, fmt.Errorf("parsing %s: multiple YAML documents are not allowed", FileName)
		}
		return Policy{}, fmt.Errorf("parsing %s after the policy: %w", FileName, err)
	}
	if err := configured.Validate(); err != nil {
		return Policy{}, fmt.Errorf("in %s: %w", FileName, err)
	}
	return configured, nil
}

// Install writes the default policy only when none exists.
func Install(rootPath string) (Policy, bool, error) {
	configured, err := Load(rootPath)
	if err == nil {
		return configured, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Policy{}, false, err
	}
	configured = Default()
	if err := Save(rootPath, configured); err != nil {
		return Policy{}, false, err
	}
	return configured, true, nil
}

// Save writes a validated policy atomically beneath the repository root.
func Save(rootPath string, configured Policy) (returnedError error) {
	if err := configured.Validate(); err != nil {
		return err
	}
	var encoded strings.Builder
	encoded.WriteString("# yaml-language-server: $schema=" + SchemaURL + "\n")
	encoder := yaml.NewEncoder(&encoded)
	encoder.SetIndent(2)
	if err := encoder.Encode(configured); err != nil {
		return fmt.Errorf("encoding %s: %w", FileName, err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("encoding %s: %w", FileName, err)
	}

	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return fmt.Errorf("opening repository root %s: %w", rootPath, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			returnedError = errors.Join(returnedError, closeErr)
		}
	}()
	if err := root.MkdirAll(path.Dir(FileName), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", path.Dir(FileName), err)
	}
	return writeAtomically(root, FileName, []byte(encoded.String()))
}

// Validate rejects policy drift and unsupported bypasses.
func (p Policy) Validate() error {
	if p.Version != Version {
		return fmt.Errorf("version %d, want %d", p.Version, Version)
	}
	if p.Comments.Mode != ModeStrict {
		return fmt.Errorf("comments.mode %q, want %q", p.Comments.Mode, ModeStrict)
	}
	if err := validateIntrinsics(p.Comments.Intrinsic); err != nil {
		return err
	}
	if err := validateGlobs("comments.generated_paths", p.Comments.GeneratedPaths); err != nil {
		return err
	}
	if err := validateGlobs("comments.vendored_paths", p.Comments.VendoredPaths); err != nil {
		return err
	}
	return validateAdapters(p.Agents.Adapters)
}

// Allows reports whether an intrinsic class is enabled.
func (p Policy) Allows(intrinsic Intrinsic) bool {
	for _, allowed := range p.Comments.Intrinsic {
		if allowed == intrinsic {
			return true
		}
	}
	return false
}

// Excludes reports whether a generated or vendored path is outside enforcement.
func (p Policy) Excludes(file string) bool {
	for _, pattern := range append(append([]string{}, p.Comments.GeneratedPaths...), p.Comments.VendoredPaths...) {
		if matches(pattern, file) {
			return true
		}
	}
	return false
}

func validateIntrinsics(values []Intrinsic) error {
	allowed := map[Intrinsic]bool{
		IntrinsicToolchain: true, IntrinsicGeneratedMarker: true, IntrinsicUpstreamLink: true,
		IntrinsicDeprecated: true, IntrinsicPublicAPI: true,
	}
	seen := map[Intrinsic]bool{}
	for _, value := range values {
		if !allowed[value] {
			return fmt.Errorf("comments.intrinsic contains unsupported class %q", value)
		}
		if seen[value] {
			return fmt.Errorf("comments.intrinsic contains %q more than once", value)
		}
		seen[value] = true
	}
	return nil
}

func validateAdapters(values []Adapter) error {
	allowed := map[Adapter]bool{
		AdapterAgents: true, AdapterClaude: true, AdapterCopilot: true,
		AdapterCursor: true, AdapterCodex: true,
	}
	seen := map[Adapter]bool{}
	for _, value := range values {
		if !allowed[value] {
			return fmt.Errorf("agents.adapters contains unsupported adapter %q", value)
		}
		if seen[value] {
			return fmt.Errorf("agents.adapters contains %q more than once", value)
		}
		seen[value] = true
	}
	return nil
}

func validateGlobs(field string, patterns []string) error {
	for _, pattern := range patterns {
		switch {
		case pattern == "":
			return fmt.Errorf("%s contains an empty pattern", field)
		case strings.Contains(pattern, `\`):
			return fmt.Errorf("%s pattern %q must use forward slashes", field, pattern)
		case strings.HasPrefix(pattern, "/"):
			return fmt.Errorf("%s pattern %q must be repository-relative", field, pattern)
		case strings.Contains("/"+pattern+"/", "/../"):
			return fmt.Errorf("%s pattern %q escapes the repository", field, pattern)
		}
		if _, err := globExpression(pattern); err != nil {
			return fmt.Errorf("%s pattern %q: %w", field, pattern, err)
		}
	}
	return nil
}

func matches(pattern, file string) bool {
	expression, err := globExpression(pattern)
	return err == nil && expression.MatchString(file)
}

func globExpression(pattern string) (*regexp.Regexp, error) {
	var expression strings.Builder
	expression.WriteString("^")
	for index := 0; index < len(pattern); index++ {
		character := pattern[index]
		switch character {
		case '*':
			if index+1 < len(pattern) && pattern[index+1] == '*' {
				index++
				if index+1 < len(pattern) && pattern[index+1] == '/' {
					index++
					expression.WriteString("(?:.*/)?")
				} else {
					expression.WriteString(".*")
				}
			} else {
				expression.WriteString("[^/]*")
			}
		case '?':
			expression.WriteString("[^/]")
		default:
			expression.WriteString(regexp.QuoteMeta(string(character)))
		}
	}
	expression.WriteString("$")
	return regexp.Compile(expression.String())
}

func writeAtomically(root *os.Root, name string, content []byte) error {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return fmt.Errorf("creating temporary name for %s: %w", name, err)
	}
	temporaryName := name + "." + hex.EncodeToString(entropy[:])
	temporary, err := root.OpenFile(temporaryName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("creating temporary file beside %s: %w", name, err)
	}
	defer func() { _ = root.Remove(temporaryName) }()
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("writing %s: %w", temporaryName, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", temporaryName, err)
	}
	if err := root.Rename(temporaryName, name); err != nil {
		return fmt.Errorf("replacing %s: %w", name, err)
	}
	return nil
}
