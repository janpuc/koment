# 0009 — Store last_seen_line as a hint, to distinguish ok from moved

Date: 2026-07-31
Status: Accepted

## Context

DESIGN.md defines four resolution statuses, and separates two of them by
position: `ok` is "excerpt found and hash matches", `moved` is "excerpt found at
a different offset".

The record format as originally written stores no position. Nothing in the
record says where the excerpt was previously found, so "a different offset"
has nothing to differ from. As specified, `ok` and `moved` are the same
observation and the implementation cannot tell them apart. This was found while
implementing `internal/anchor`, not by reading the document.

Three ways out: drop `moved`, invent a position-free meaning for it, or record a
position. The first two lose real signal — an excerpt that has shifted is a
weaker confirmation than one that has not moved at all, and a reviewer wants to
know which they are looking at.

ADR 0003 says "line numbers are never stored as an anchor". That constraint is
about what resolution *depends on*, not about what a record may remember.

## Decision

Records carry `last_seen_line`: the 1-based line where the excerpt was found the
last time the annotation was written or confirmed.

It is a hint and never an anchor:

- Resolution searches for the excerpt text alone. `last_seen_line` is not
  consulted while searching and cannot cause a match or a miss.
- It is read only after a match, to choose between `ok` and `moved`.
- If it is absent or zero, resolution reports `ok`. An unknown previous position
  is not evidence of movement.

`koment add` refuses to create an anchor whose excerpt appears more than once in
the file. Ambiguity is rejected at creation, where a human can fix it, rather
than guessed at during resolution.

## Consequences

- `moved` becomes a real, testable status instead of an unreachable one.
- The distinction degrades safely. Corrupt or delete `last_seen_line` and every
  annotation still resolves; the worst outcome is `ok` reported where `moved`
  was true. No annotation is ever lost or mismatched because of this field.
- The field goes stale on any edit above the excerpt, so a `moved` result is
  common and carries no urgency. Only `drifted` and `orphaned` fail the build.
- Rewriting `last_seen_line` on confirmation means `koment check` could produce
  a diff. It does not: `check` is read-only, and only `add` writes.
- A reader who knows ADR 0003 will see a line number in the record and suspect a
  contradiction. That is exactly why this ADR exists.

## Alternatives rejected

- **Drop `moved`; keep ok, drifted, orphaned.** Simplest, and genuinely
  defensible. Rejected because "the code you annotated is still here but has
  shifted" is information a reviewer wants, and because the three remaining
  statuses would then be derivable without storing anything — a design that
  cannot distinguish a stable anchor from a drifting one has less to say over
  time, and drift statistics are the evidence ADR 0003 asks for before adopting
  symbol scope.
- **Byte offset instead of line number.** More precise, and equally cheap to
  compare. Loses on utility: every consumer — `koment show`, the MCP payload, a
  reviewer reading a diff — wants `resolve.go:42`, and a byte offset would have
  to be converted to a line for all of them anyway.
- **`moved` means "the excerpt occurs more than once".** Position-free, needs no
  new field. Rejected because that is ambiguity, not movement; it would report
  `moved` for an annotation that has not moved at all, which makes the status a
  lie.
- **Recompute position from git history.** Correct in principle and needs no new
  field, but it makes resolution depend on a git binary, on history not being
  squashed, and on the file having been committed at all. Reading a checkout
  must not require any of that.
- **Store the surrounding context lines instead** (Microsoft Research's keyword
  approach). A stronger anchor, and the natural upgrade if excerpt matching
  proves too brittle. Out of scope here: this ADR only needs to separate two
  statuses, and adding a second anchoring mechanism to do it would be a much
  larger change than the problem warrants.
