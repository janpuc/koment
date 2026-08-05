# 0116 — Stop reporting a stale line number as a status

Date: 2026-08-05

Status: Accepted

Amends [ADR 0101](0101-fail-ambiguous-anchor-resolution.md).

## Context

Resolution searched the file for the excerpt and then compared where it found it
with `last_seen_line`:

```go
status := StatusMoved
if found.line == annotation.Anchor.LastSeenLine {
    status = StatusOK
}
```

That is the whole of `moved`. The excerpt had already resolved uniquely. The
annotation was correct, `koment check` passed, and the only thing that differed
was a stored integer.

An annotation therefore already follows its code. Nothing has to move it:
resolution finds it wherever the excerpt now is, on every read. The only thing
that does not move is the cached line, and ADR 0101 and `DESIGN.md` both already
said that line "is descriptive metadata. It never selects a candidate."

So `moved` reported a cache miss rather than a fact about the code, and it was
the ordinary case rather than the exception: 33 of 68 annotations in this
repository and 13 of 78 in the first repository to adopt koment. A status that
describes the majority carries no information.

It also could not be acted on. ADR 0114 had already removed it from editor
diagnostics because marking healthy code teaches readers to ignore markers, and
a linter has nothing to say about it either. It survived only in listings, where
it added a column that no reader could use.

## Decision

Remove `moved`. A unique resolution is `ok`, wherever the excerpt was found.

Four statuses remain. `ok` passes; `ambiguous`, `drifted` and `orphaned` fail.
Every koment status is now something a reader must act on, and `koment check`
failing means exactly one thing.

`last_seen_line` stays in the record. It is written when an annotation is
created or reanchored, pairs with the `git` block to say where the prose was
last confirmed, and is no longer read to decide anything.

koment does not rewrite records during a read. Refreshing `last_seen_line`
whenever an excerpt resolved elsewhere would make `koment check` modify the
working tree, so a check in CI would either fail on its own bookkeeping or
commit to the repository it was asked to inspect.

## Consequences

- `koment check` output is shorter and every line in it is actionable.
- Nothing reports that a recorded line has gone stale, because nothing needs to.
- `last_seen_line` is now write-only. It has to justify itself as provenance or
  be removed by a later decision; this one does not remove it, because that is a
  record format change and this is not.
- Metrics lose the `moved` series. A dashboard panel referencing it shows no
  data rather than zero, which is the usual cost of removing a label value.
- Published `annotations.json` no longer contains `"status": "moved"`. It is
  regenerated on every publish, so nothing needs migrating.
- Anyone who read `moved` as "reanchor this to refresh provenance" loses that
  prompt. That reading was never enforced and the refresh was never required.

## Alternatives rejected

- **Keep `moved` and refresh `last_seen_line` automatically on resolution.**
  The obvious fix, and it makes reads write: `koment check` would dirty the
  working tree, CI would fail on a diff it created itself, and a read-only
  surface like the static export could not do it at all.
- **Add `koment sync` to refresh stale lines deliberately.** Honest, and it
  keeps reads pure, but it exists only to silence a status that nobody acts on.
  Removing the status removes the need for the command.
- **Demote `moved` to a field on the resolution rather than a status.** Keeps
  the information for anyone who wants it, but every surface would still have to
  decide whether to show it, and the answer everywhere was no.
- **Remove `last_seen_line` as well.** Tempting, since nothing reads it now, and
  it would leave the anchor holding only what resolution uses. It changes the
  record format and invalidates every existing annotation against the published
  schema, which is a larger decision than this one.
- **Keep it because five statuses were documented.** The documentation is ours
  to correct, and a status that the tool itself had already stopped showing in
  editors was documentation describing something users could not see.
