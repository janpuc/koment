# 0012 — Make re-anchoring a command, not a manual edit

Date: 2026-07-31
Status: Accepted

## Context

koment reports drift and then abandons the reader. `check` says an annotation
no longer resolves; nothing helps fix it. The only route back to green is to
open `.koment/annotations/<path>.yaml`, edit `excerpt`, compute a SHA-256 of the
new text by hand, paste it into `excerpt_sha256`, and correct `last_seen_line`.

This is not hypothetical. Within an hour of the store existing, a signature
change during the HTTP transport work drifted an annotation on `cli.Run`, and
fixing it required exactly that sequence — including running `shasum -a 256` in
a scratch shell.

That workflow is bad in a way that matters more than inconvenience. ADR 0003
accepts drift noise as the price of excerpt anchoring, on the assumption that
re-confirming is cheap. It is not cheap today, and the rational response to an
expensive fix is to delete the annotation instead. A design that makes deleting
the annotation the path of least resistance defeats the entire project.

Storing a hash the user must compute by hand is also a footgun: nothing stops a
wrong hash being written, and `store.Load` correctly refuses to load the record
afterwards, so a typo turns drift into a hard parse failure.

## Decision

Add `koment reanchor`:

```
koment reanchor <id> [--excerpt <text>] [--file <path>]
```

- `--excerpt` re-points a drifted annotation at new text in its current file.
- `--file` moves an annotation to a different path, for the orphaned case where
  a source file was renamed.
- Both may be combined, for a rename that also changed the annotated code.

The annotation's ULID does not change. That is the point: the annotation is the
same annotation, still carrying its original `created` date and its history in
git. Re-anchoring is an edit, not a replacement.

`excerpt_sha256` and `last_seen_line` are recomputed, never typed. The new
excerpt is validated the same way `add` validates one: absent or ambiguous is
refused, so re-anchoring cannot introduce an anchor that `add` would reject.

## Consequences

- Drift becomes cheap to resolve honestly, so the incentive to delete the
  annotation instead disappears. This is the whole justification.
- Hashes are never hand-written, which removes a class of corrupt records.
- The v1 surface grows a fifth command. That is a real cost against keeping the
  tool small, and it is accepted because `check` without `reanchor` is a
  diagnosis with no treatment.
- Re-anchoring is deliberately not automatic. `check` still only reports;
  nothing rewrites an anchor without a person naming the annotation and the new
  excerpt. An automatic re-anchor that guessed would be exactly the silent
  re-attachment ADR 0003 rejected.
- Looking an annotation up by id alone means scanning every record. At the scale
  of one repository's annotations this is a non-issue, and it keeps the command
  line short enough to paste from `check` output.

## Alternatives rejected

- **Leave it manual.** What v1 shipped. Rejected on evidence: it took under an
  hour of real use to hit, and the workaround involves hand-computing a hash the
  store then validates strictly.
- **`--fix` on `check`, re-anchoring everything it can.** Convenient, and wrong.
  It would re-attach annotations to whatever text looked closest without a
  person confirming that the rationale still applies. ADR 0003 rejected silent
  re-attachment as worse than reporting drift, and doing it in bulk is that
  failure at scale.
- **Delete and re-add.** Needs no new command. Rejected because it mints a new
  ULID and a new `created` date, breaking the "stable across edits, never
  reused" property in DESIGN.md and severing the annotation from its own
  history — the record would claim to be younger than the thinking in it.
- **Identify the annotation by file and excerpt instead of id.** Avoids the
  scan. Rejected because the drifted excerpt is precisely the thing that no
  longer exists, so it is a poor handle; the id is stable and `check` already
  prints it.
- **An interactive editor session (`$EDITOR` on the record).** Flexible, and
  reintroduces every hand-editing hazard this ADR exists to remove.
