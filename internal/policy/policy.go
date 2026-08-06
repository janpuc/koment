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

	"github.com/koment-dev/koment/internal/api"
)

const (
	// APIVersion is matched exactly, exactly as an annotation record's is.
	APIVersion = api.Version

	// KindPolicy is the resource kind of a repository policy.
	KindPolicy = "Policy"

	ModeStrict = "strict"
	FileName   = ".koment/policy.yaml"
	SchemaURL  = api.SchemaBase + "policy.schema.json"
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

// Principle names one extra rule the generated agent contract states. A
// principle is a claim a reviewer can check, not a preference.
type Principle string

const (
	PrincipleBackCompatEvidence Principle = "back-compat-evidence"
)

var Principles = []Principle{PrincipleBackCompatEvidence}

var principleText = map[Principle]string{
	PrincipleBackCompatEvidence: "A back-compatibility claim needs evidence: a migration path the binary performs, " +
		"or an ADR naming the version the old shape was cut off at. Without either, the change is breaking " +
		"and its commit subject says so with `feat!:`.",
}

// Adapter names one generated agent instruction surface.
type Adapter string

const (
	AdapterAgents   Adapter = "agents"
	AdapterClaude   Adapter = "claude"
	AdapterCopilot  Adapter = "copilot"
	AdapterCursor   Adapter = "cursor"
	AdapterCodex    Adapter = "codex"
	AdapterOpencode Adapter = "opencode"
)

// Policy is the repository enforcement contract, shaped like every other
// committed koment resource (ADR 0121).
type Policy struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       Spec   `yaml:"spec"`
}

// Spec is everything the repository decided. A policy has no metadata because
// it is a singleton with no identity of its own: the file path is the name.
type Spec struct {
	Comments CommentsPolicy `yaml:"comments"`
	Agents   AgentsPolicy   `yaml:"agents"`
}

// CommentsPolicy configures strict classification and repository exclusions.
type CommentsPolicy struct {
	Mode           string      `yaml:"mode"`
	Intrinsic      []Intrinsic `yaml:"intrinsic"`
	GeneratedPaths []string    `yaml:"generatedPaths,omitempty"`
	VendoredPaths  []string    `yaml:"vendoredPaths,omitempty"`
}

// AgentsPolicy selects generated instruction adapters and the principles they
// state.
type AgentsPolicy struct {
	Adapters   []Adapter   `yaml:"adapters"`
	Principles []Principle `yaml:"principles,omitempty"`
}

// Default returns the strict policy installed for a new repository.
func Default() Policy {
	return Policy{
		APIVersion: APIVersion,
		Kind:       KindPolicy,
		Spec: Spec{
			Comments: CommentsPolicy{
				Mode: ModeStrict,
				Intrinsic: []Intrinsic{
					IntrinsicToolchain, IntrinsicGeneratedMarker, IntrinsicUpstreamLink,
					IntrinsicDeprecated, IntrinsicPublicAPI,
				},
				GeneratedPaths: []string{"**/*.gen.go", "**/*.generated.go"},
				VendoredPaths:  []string{"vendor/**"},
			},
			Agents: AgentsPolicy{
				Adapters: []Adapter{
					AdapterAgents, AdapterClaude, AdapterCopilot, AdapterCursor, AdapterCodex,
					AdapterOpencode,
				},
				Principles: []Principle{PrincipleBackCompatEvidence},
			},
		},
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
	configured, upgraded, err := decode(content)
	if err != nil {
		return Policy{}, err
	}
	if err := configured.Validate(); err != nil {
		return Policy{}, fmt.Errorf("in %s: %w", FileName, err)
	}
	if upgraded {
		if err := writeTo(root, configured); err != nil {
			return Policy{}, err
		}
	}
	return configured, nil
}

type policyShape struct {
	APIVersion string `yaml:"apiVersion"`
	Version    *int   `yaml:"version"`
}

func decode(content []byte) (Policy, bool, error) {
	var shape policyShape
	if err := yaml.Unmarshal(content, &shape); err != nil {
		return Policy{}, false, fmt.Errorf("parsing %s: %w", FileName, err)
	}
	switch {
	case shape.APIVersion == APIVersion:
		var configured Policy
		if err := decodeOneDocument(content, &configured); err != nil {
			return Policy{}, false, err
		}
		return configured, false, nil
	case shape.APIVersion != "":
		return Policy{}, false, fmt.Errorf(
			"incompatible %s: apiVersion %q is not supported; this binary reads %s (ADR 0121)",
			FileName, shape.APIVersion, APIVersion)
	case shape.Version != nil && *shape.Version == LegacyVersion:
		var legacy legacyPolicy
		if err := decodeOneDocument(content, &legacy); err != nil {
			return Policy{}, false, err
		}
		return upgradeLegacy(legacy), true, nil
	default:
		return Policy{}, false, fmt.Errorf(
			"incompatible %s: no apiVersion; a koment policy starts with `apiVersion: %s` (ADR 0121)",
			FileName, APIVersion)
	}
}

func decodeOneDocument(content []byte, into any) error {
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	decoder.KnownFields(true)
	if err := decoder.Decode(into); err != nil {
		return fmt.Errorf("parsing %s: %w", FileName, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("parsing %s: multiple YAML documents are not allowed", FileName)
		}
		return fmt.Errorf("parsing %s after the policy: %w", FileName, err)
	}
	return nil
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
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return fmt.Errorf("opening repository root %s: %w", rootPath, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			returnedError = errors.Join(returnedError, closeErr)
		}
	}()
	return writeTo(root, configured)
}

func writeTo(root *os.Root, configured Policy) error {
	encoded, err := encode(configured)
	if err != nil {
		return err
	}
	if err := root.MkdirAll(path.Dir(FileName), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", path.Dir(FileName), err)
	}
	return writeAtomically(root, FileName, encoded)
}

func encode(configured Policy) ([]byte, error) {
	var encoded strings.Builder
	encoded.WriteString("# yaml-language-server: $schema=" + SchemaURL + "\n")
	encoder := yaml.NewEncoder(&encoded)
	encoder.SetIndent(2)
	if err := encoder.Encode(configured); err != nil {
		return nil, fmt.Errorf("encoding %s: %w", FileName, err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("encoding %s: %w", FileName, err)
	}
	return []byte(encoded.String()), nil
}

// Validate rejects policy drift and unsupported bypasses.
func (p Policy) Validate() error {
	if p.APIVersion != APIVersion {
		return fmt.Errorf("apiVersion %q, want %q", p.APIVersion, APIVersion)
	}
	if p.Kind != KindPolicy {
		return fmt.Errorf("kind %q, want %q", p.Kind, KindPolicy)
	}
	if p.Spec.Comments.Mode != ModeStrict {
		return fmt.Errorf("spec.comments.mode %q, want %q", p.Spec.Comments.Mode, ModeStrict)
	}
	if err := validateIntrinsics(p.Spec.Comments.Intrinsic); err != nil {
		return err
	}
	if err := validateGlobs("spec.comments.generatedPaths", p.Spec.Comments.GeneratedPaths); err != nil {
		return err
	}
	if err := validateGlobs("spec.comments.vendoredPaths", p.Spec.Comments.VendoredPaths); err != nil {
		return err
	}
	if err := validatePrinciples(p.Spec.Agents.Principles); err != nil {
		return err
	}
	return validateAdapters(p.Spec.Agents.Adapters)
}

// States returns the wording of every principle this policy enables, in the
// order the vocabulary declares them so that a regenerated contract does not
// diff against itself.
func (p Policy) States() []string {
	stated := make([]string, 0, len(p.Spec.Agents.Principles))
	for _, principle := range Principles {
		for _, enabled := range p.Spec.Agents.Principles {
			if enabled == principle {
				stated = append(stated, principleText[principle])
			}
		}
	}
	return stated
}

// Allows reports whether an intrinsic class is enabled.
func (p Policy) Allows(intrinsic Intrinsic) bool {
	for _, allowed := range p.Spec.Comments.Intrinsic {
		if allowed == intrinsic {
			return true
		}
	}
	return false
}

// Excludes reports whether a generated or vendored path is outside enforcement.
func (p Policy) Excludes(file string) bool {
	for _, pattern := range append(append([]string{}, p.Spec.Comments.GeneratedPaths...), p.Spec.Comments.VendoredPaths...) {
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
			return fmt.Errorf("spec.comments.intrinsic contains unsupported class %q", value)
		}
		if seen[value] {
			return fmt.Errorf("spec.comments.intrinsic contains %q more than once", value)
		}
		seen[value] = true
	}
	return nil
}

func validatePrinciples(values []Principle) error {
	seen := map[Principle]bool{}
	for _, value := range values {
		if _, known := principleText[value]; !known {
			return fmt.Errorf("spec.agents.principles contains unsupported principle %q", value)
		}
		if seen[value] {
			return fmt.Errorf("spec.agents.principles contains %q more than once", value)
		}
		seen[value] = true
	}
	return nil
}

func validateAdapters(values []Adapter) error {
	allowed := map[Adapter]bool{
		AdapterAgents: true, AdapterClaude: true, AdapterCopilot: true,
		AdapterCursor: true, AdapterCodex: true, AdapterOpencode: true,
	}
	seen := map[Adapter]bool{}
	for _, value := range values {
		if !allowed[value] {
			return fmt.Errorf("spec.agents.adapters contains unsupported adapter %q", value)
		}
		if seen[value] {
			return fmt.Errorf("spec.agents.adapters contains %q more than once", value)
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
