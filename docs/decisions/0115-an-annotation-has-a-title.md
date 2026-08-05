# 0115 — Give an annotation a title

Date: 2026-08-05

Status: Accepted

## Context

ADR 0114 gave rationale two surfaces: an inline gloss saying that rationale
exists, and a panel where the body is read. The gloss rendered the body itself,
flattened to one line and cut at a configurable width.

First real use rejected that. A body is a paragraph, so every gloss ended mid
sentence. A line that is always cut is worse than no line: it reads as damage
rather than as a summary, and it invites the reader to widen a setting that
cannot help, because the next body is longer again. Truncation is not a display
bug to tune. It is what happens when a surface is asked to show something whose
length it does not control.

The gloss also carried a speech-bubble emoji next to a status word, which put
three decorations on a line whose job was to be quiet.

## Decision

An annotation carries an optional `title`: one line, at most 72 characters,
validated at write time.

The limit is the point. A title short enough to render beside code is a title
that never needs shortening, so no surface has to decide where to cut. `koment`
refuses a longer one rather than accepting it and truncating later.

A record written before titles existed still has to show something, so
`Annotation.Headline()` falls back to the first sentence of the body, shortened
at a word boundary. The fallback is computed on read and never written back. A
derived title stored in the record would be a second copy of the body, free to
drift from it, which is the failure this project exists to prevent.

Every surface shows the headline: the inline gloss, the hover, the panel, the
code lens, the web view, the static export and the search index.

The inline gloss shows the headline and nothing else — no emoji, and the status
only when it is not `ok`. The body is reached by clicking, hovering, or reading
the panel.

`koment.inlineMaxLength` is removed. It existed to tune truncation that no
longer happens.

## Consequences

- The gloss is a fixed, readable length, and the reader chooses when to see more.
- Annotations written before this change render a derived headline that is
  usually the first sentence, which is what a title would have said anyway.
- Authors gain a decision: a good title is work, and a bad one is worse than the
  derived fallback because it hides the sentence it replaced.
- A record carrying a title is rejected by koment 0.4.0 and earlier, because the
  decoder refuses unknown fields. Reading old records stays fine.
- The 72-character limit is a judgement, not a measurement. It fits beside code
  at common widths; it is not derived from anything.

## Alternatives rejected

- **Keep truncating, raise the limit.** No change to the record and one setting
  to tune, but the next body is always longer. The reader still meets a cut line
  and now has a setting that implies otherwise.
- **Derive the title and store it.** Removes the authoring cost, but puts a copy
  of the body's first sentence in the record where it can drift from the body it
  summarises. Deriving on read costs nothing and cannot go stale.
- **Require a title on every annotation.** Consistent, and it would make the
  fallback unnecessary, but it invalidates every existing record and taxes the
  quick note that koment exists to make cheap.
- **Wrap the body across several inline decorations.** Shows everything with no
  new field, but VS Code renders decorations per line, so the prose would
  interleave with the code it describes.
- **Show only the kind inline and move all prose to the panel.** Quietest
  editor, and it needs no new field, but the gloss then says only that something
  exists, which is not enough to decide whether to open it.
