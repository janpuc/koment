# 0006 — Depend on go.yaml.in/yaml/v3 for the record format

Date: 2026-07-31
Status: Accepted

## Context

ADR 0002 puts annotations in files that must be reviewable in a pull request.
DESIGN.md specifies YAML for the record format, and the reason is the `body`
field: it holds prose, often several sentences, and it is the thing a reviewer
actually reads. YAML block scalars can hold that prose as plain lines that diff
one line at a time.

Note that YAML gives this only if the writer supplies the line breaks. Measured
against v3.0.5: the emitter is initialised with `best_width: -1`, which
`yaml_emitter_analyze_version_directive` turns into `1<<31 - 1`, and the public
API exposes no `SetWidth`. A long single-line body is therefore emitted as one
long line whatever the scalar style. koment hard-wraps body text itself, when an
annotation is authored — see `store.WrapProse`.

The Go standard library has no YAML parser, so this is a dependency decision
under AGENTS.md §10.

The obvious candidate, `gopkg.in/yaml.v3`, is no longer maintained. Its
repository states:

> This repository was archived by the owner on Apr 1, 2025. It is now read-only.

The YAML organization took over the project after that. Verified against the
module proxy on 2026-07-31:

| module | latest | transitive deps | status |
|---|---|---|---|
| `gopkg.in/yaml.v3` | v3.0.1 | 0 | archived, read-only |
| `go.yaml.in/yaml/v3` | v3.0.5 | 0 | frozen; security fixes only |
| `go.yaml.in/yaml/v4` | v4.0.0-rc.6 | 0 | active development, no stable release |

## Decision

Depend on `go.yaml.in/yaml/v3` at v3.0.5.

Confine the dependency to `internal/store`. No other package imports it, and no
YAML type appears in a signature outside that package.

## Consequences

- One dependency, zero transitive dependencies. The whole module graph for the
  store is one package by one maintainer organization.
- "Frozen, security fixes only" is a feature here, not a warning. koment needs a
  YAML parser that behaves in five years exactly as it does today; a parser that
  stops changing is a parser that stops surprising us.
- v4 will eventually be the maintained line. Because the dependency is confined
  to `internal/store` and never leaks into a signature, migrating is one
  package's problem. Do not migrate before v4 has a stable release.
- Annotation bodies stay readable in a diff, which is what ADR 0002 was for —
  but only because koment wraps them on the way in. Wrapping happens when an
  annotation is authored, never when a record is saved, so the store keeps its
  promise to round-trip exactly what it was given.

## Alternatives rejected

- **JSON via `encoding/json`.** Zero dependencies, and genuinely tempting. It
  loses on the one axis that matters: JSON has no multi-line string, so a
  multi-sentence `body` becomes a single escaped line no matter how it is
  wrapped beforehand. That makes annotation changes unreviewable in a PR and
  defeats the purpose of ADR 0002. YAML's block scalars are the specific feature
  being bought here.
- **`gopkg.in/yaml.v3`.** Archived and read-only since April 2025. Taking a new
  dependency on an explicitly unmaintained package fails AGENTS.md §10's
  "long-lived" bar on day one.
- **`go.yaml.in/yaml/v4`.** The line that will get new features. Rejected for
  now because it has no stable release — `v4.0.0-rc.6` is the newest tag — and
  koment's record format needs none of those features.
- **`sigs.k8s.io/yaml`.** Converts YAML to JSON and unmarshals with
  `encoding/json`, so struct tags stay `json:`. Rejected because the conversion
  discards the node model, which forecloses ever preserving comments or key
  order on rewrite, and it pulls its own YAML dependency underneath anyway.
- **Hand-rolling a YAML subset.** YAML is large and its edge cases — block
  scalar indentation, quoting, escapes — are exactly where a hand-rolled parser
  silently corrupts data. Corrupting an annotation is the worst failure this
  project has, per AGENTS.md §5.
