# 0114 — Show rationale in a panel, and stop diagnosing what is healthy

Date: 2026-08-04
Status: Accepted

## Context

The first real use of the published extension produced two complaints, and both
are about presentation rather than correctness.

An annotation body is a paragraph. The editor rendered it as a single inline
decoration after the annotated line, squashed to one line and truncated at 100
characters, with the full text reachable only by hovering. Rationale that has to
be hovered to be read is rationale most people never read, which defeats the
reason it was written down.

VS Code cannot render multi-line text beside code without putting it in the
document. ADR 0110 forbids that: the extension "never inserts virtual comments
into the source buffer", because prose in the buffer is the comment this project
exists to remove. Decorations, inlay hints and code lenses are all single-line
surfaces. There is no inline answer.

Separately, an annotation whose status is `moved` was published as an LSP
diagnostic with severity Information, which VS Code draws as a blue squiggle
under the code. `moved` means the anchor resolved uniquely and the recorded line
changed — the annotation is correct and `koment check` passes. This repository
carries 26 of them, so the editor underlined healthy code in most files. A
marker that fires on a non-problem teaches people to ignore markers.

## Decision

Rationale gets two surfaces, each doing what it is good at.

The **inline gloss** stays: one decoration after the annotated line carrying the
kind, the status when it is not `ok`, and as much body as fits. Its job is to
say that rationale exists here, not to deliver it. A toggle removes the
truncation for readers who want the whole line inline.

A **koment panel** lists every annotation in the active file with its complete
body, its kind, its status and its line, and reveals the line when an entry is
chosen. This is where prose is read.

The panel renders prose rather than tree rows. A tree item is a single line, so
a tree would have reproduced the truncation this decision exists to remove.

`moved` stops being a diagnostic. It is reported in the inline gloss and in the
panel, where it belongs, and the diagnostics list keeps only what fails
`koment check` — `ambiguous`, `drifted`, `orphaned` — plus prohibited comments.
A koment diagnostic now means the build is red.

Comment detection stays Go-only and moves behind a detector interface, so a
second language is a new implementation rather than a rewrite of
`commentpolicy`. No language gains detection in this change.

## Consequences

- Rationale is readable without hovering, and the panel scales to bodies of any
  length.
- The extension gains a webview, which is more surface than a decoration and
  needs its own content security policy.
- Diagnostics become a reliable signal: everything koment reports in the
  Problems panel would fail CI.
- A `moved` annotation is less prominent than it was. That is the point, and it
  means provenance drift is now noticed in the gloss or the panel rather than by
  a squiggle demanding attention.
- The detector interface adds an indirection that has exactly one implementation
  until a second language arrives.
- The panel duplicates what the static site already renders. They stay separate
  because one follows the cursor in an editor and the other is a published
  snapshot.

## Alternatives rejected

- **Expand the inline decoration to the full body.** One toggle and no new
  surface, but a decoration is one line: a paragraph runs off the right edge and
  is unreadable. It solves the truncation without solving the reading.
- **Insert the body into the buffer as virtual lines.** The only way to get
  multi-line inline text, and precisely what ADR 0110 forbids — prose in the
  source file is the thing koment removes.
- **A tree view instead of a webview.** Cheaper and more native, but a tree item
  renders one line, so full bodies would need one child row per wrapped line.
  That is the same truncation problem wearing a different control.
- **Drop the inline gloss and keep only the panel.** The cleanest editor, but it
  removes the at-a-glance signal that a line has rationale, which is what makes
  anyone open the panel at all.
- **Keep `moved` as a Hint instead of Information.** Quieter, and a smaller
  change, but a hint is still a marker on code that is not wrong; the honest
  move is to stop marking it.
- **Add regex comment detection for other languages now.** Broad coverage
  quickly, but a scanner that cannot tell a comment from a string literal
  produces false prohibited-comment failures, and those block CI. A wrong gate
  is worse than an absent one.
