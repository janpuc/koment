# 0002 — Store annotations out of band, mirrored per source file

Date: 2026-07-31
Status: Accepted

## Context

The founding constraint is that annotations must not live in the source. They
still have to be version-controlled, reviewable in a pull request, and diffable
alongside the change that motivated them.

## Decision

Annotations live in `.koment/annotations/<mirrored source path>.yaml`, one file
per annotated source file, committed to the repository.

## Consequences

- A change to code and a change to its annotation appear in the same pull
  request and can be reviewed together.
- Diffs stay local: editing one file's annotations cannot conflict with another's.
- A source file rename shows up as a rename on both sides, which is a visible,
  fixable signal rather than silent drift.
- The tree is duplicated, which is mild clutter. Accepted.
- Annotations survive losing every external service. There is no server in the
  read path.

## Alternatives rejected

- **A single store file** (`.koment/annotations.jsonl` or a SQLite database).
  Every annotation edit touches one file, so concurrent work merge-conflicts
  constantly, and a binary store is unreviewable in a PR — which defeats the
  purpose.
- **Git notes.** Genuinely out-of-band and git-native, but notes attach to
  commits, not to file regions; they are not fetched by default, most UIs never
  show them, and almost no developer knows they exist.
- **A sidecar file next to each source file** (`resolve.go.koment`). Pollutes
  the source tree, trips globs, build tooling and language servers, and makes
  `ls` noisy.
- **An external service** (see also ADR 0004). Adds an availability dependency
  to reading your own codebase, and annotations stop being reviewable.
