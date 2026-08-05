package store

// LegacyRecordVersion is the only value the pre-v1alpha `version` field ever
// carried.
const LegacyRecordVersion = 1

// legacyRecord is the record shape koment wrote before ADR 0119. It exists
// only so that a repository written by an older binary can be read once and
// rewritten in the current shape.
//
// Deprecated: delete legacyRecord and upgradeLegacy in the release after
// 1.0.0. By then every repository that a 1.0.x binary has read is already in
// the current shape, and a record still carrying `version: 1` should be
// refused rather than silently upgraded.
type legacyRecord struct {
	Version int          `yaml:"version"`
	ID      string       `yaml:"id"`
	File    string       `yaml:"file"`
	Kind    Type         `yaml:"kind"`
	Title   string       `yaml:"title,omitempty"`
	Body    string       `yaml:"body"`
	Created Timestamp    `yaml:"created"`
	Anchor  legacyAnchor `yaml:"anchor"`
	Git     *legacyGit   `yaml:"git,omitempty"`
	Author  Author       `yaml:"author"`
	Policy  *Policy      `yaml:"policy,omitempty"`
}

// Deprecated: see legacyRecord.
type legacyGit struct {
	Commit  string `yaml:"commit"`
	Path    string `yaml:"path"`
	Line    int    `yaml:"line,omitempty"`
	EndLine int    `yaml:"end_line,omitempty"`
}

// Deprecated: see legacyRecord.
type legacyAnchor struct {
	Scope        Scope  `yaml:"scope"`
	Excerpt      string `yaml:"excerpt,omitempty"`
	Before       string `yaml:"before,omitempty"`
	After        string `yaml:"after,omitempty"`
	LastSeenLine int    `yaml:"last_seen_line,omitempty"`
}

// upgradeLegacy maps a v1 record onto the current shape. It moves
// last_seen_line into status because a line is observed rather than authored,
// and it records no resolution: the reader that performed the upgrade did not
// resolve anything.
//
// Deprecated: see legacyRecord.
func upgradeLegacy(record legacyRecord) Annotation {
	return Annotation{
		APIVersion: APIVersion,
		Kind:       KindAnnotation,
		Metadata:   Metadata{ID: record.ID, Created: record.Created},
		Spec: Spec{
			Target: Target{File: record.File},
			Type:   record.Kind,
			Title:  record.Title,
			Body:   record.Body,
			Anchor: Anchor{
				Scope:   record.Anchor.Scope,
				Excerpt: record.Anchor.Excerpt,
				Before:  record.Anchor.Before,
				After:   record.Anchor.After,
			},
			Author: record.Author,
			Git:    upgradeLegacyGit(record.Git),
			Policy: record.Policy,
		},
		Status: Status{LastSeenLine: record.Anchor.LastSeenLine},
	}
}

// Deprecated: see legacyRecord.
func upgradeLegacyGit(context *legacyGit) *GitContext {
	if context == nil {
		return nil
	}
	return &GitContext{
		Commit: context.Commit, Path: context.Path, Line: context.Line, EndLine: context.EndLine,
	}
}
