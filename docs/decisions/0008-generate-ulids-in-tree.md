# 0008 — Generate ULIDs in tree rather than depend on oklog/ulid

Date: 2026-07-31
Status: Accepted

## Context

DESIGN.md gives every annotation a ULID: stable across edits, never reused.
`github.com/oklog/ulid/v2` is the established Go implementation and exists at
v2.1.2, verified on the module proxy 2026-07-31.

What koment actually needs from ULID is narrow. It generates an id when an
annotation is created and writes it to YAML. It never parses one back into a
timestamp, never sorts by it, never compares two except as strings, and never
accepts one as input. The spec is fixed: 48 bits of millisecond timestamp and 80
bits of randomness, rendered as 26 characters of Crockford base32.

## Decision

Generate ULIDs in `internal/store`, using `crypto/rand` and a Crockford base32
alphabet. Roughly forty lines, generation only — no parsing, no decoding, no
monotonic-within-millisecond entropy.

## Consequences

- No third dependency for a format that is a paragraph of specification and will
  never change.
- Encoding correctness becomes ours to prove, so it gets a test asserting length,
  alphabet, lexicographic ordering across timestamps, and uniqueness under a
  tight generation loop.
- No monotonic guarantee within a single millisecond. Two annotations created in
  the same millisecond may sort in either order relative to each other. koment
  orders annotations by file position and creation date, never by id, so this
  costs nothing. Anything that later needs strict creation ordering must not
  reach for the id.
- If koment ever needs to *parse* ULIDs — extracting creation time from an id,
  say — that is the point to reconsider `oklog/ulid` rather than grow a decoder
  here.

## Alternatives rejected

- **`github.com/oklog/ulid/v2`.** Correct, well-used, small. Rejected because
  AGENTS.md §10 asks the standard library first, and this is forty lines of
  generation against a frozen spec with no edge cases we exercise. A dependency
  is a liability that has to be retired eventually; this one does not earn that.
- **UUIDv4 via `crypto/rand`.** Also dependency-free and needs no encoder. Loses
  the property DESIGN.md wanted: a ULID sorts by creation time, so a plain
  listing of annotation ids is chronological and an id carries a rough age.
- **UUIDv7.** Time-ordered like ULID, and Go 1.26 has no standard-library
  generator for it, so it costs the same in-tree code as ULID while producing a
  less readable identifier — hyphens and hex against ULID's 26 case-insensitive
  characters that survive a double-click.
- **A monotonic counter or content hash.** A counter is not stable across
  concurrent work and collides on merge. A content hash changes when the body is
  edited, which breaks the "stable across edits" requirement outright.
