package policy

// LegacyVersion is the only value the pre-v1alpha `version` field ever
// carried.
const LegacyVersion = 1

// legacyPolicy is the flat shape koment wrote before ADR 0121.
//
// Deprecated: delete legacyPolicy and upgradeLegacy in the release after
// 1.0.0, once every repository a 1.0.x binary has read carries the current
// shape.
type legacyPolicy struct {
	Version  int                  `yaml:"version"`
	Comments legacyCommentsPolicy `yaml:"comments"`
	Agents   AgentsPolicy         `yaml:"agents"`
}

// Deprecated: see legacyPolicy.
type legacyCommentsPolicy struct {
	Mode           string      `yaml:"mode"`
	Intrinsic      []Intrinsic `yaml:"intrinsic"`
	GeneratedPaths []string    `yaml:"generated_paths,omitempty"`
	VendoredPaths  []string    `yaml:"vendored_paths,omitempty"`
}

// Deprecated: see legacyPolicy.
func upgradeLegacy(configured legacyPolicy) Policy {
	return Policy{
		APIVersion: APIVersion,
		Kind:       KindPolicy,
		Spec: Spec{
			Comments: CommentsPolicy{
				Mode:           configured.Comments.Mode,
				Intrinsic:      configured.Comments.Intrinsic,
				GeneratedPaths: configured.Comments.GeneratedPaths,
				VendoredPaths:  configured.Comments.VendoredPaths,
			},
			Agents: configured.Agents,
		},
	}
}
