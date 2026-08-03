# 0101 — Fail deterministic resolution when an anchor is ambiguous

Date: 2026-08-03
Status: Accepted

## Context

An exact excerpt is language-independent and easy to review, but the same text
can appear more than once. v0.2 rejects ambiguity when creating an annotation
yet treats a later duplicate as `ok` when one occurrence remains at
`last_seen_line`. The CLI can warn about the occurrence count while MCP and the
UI omit it. A line that was defined as a hint has therefore become an implicit
identity choice.

The project is useful only when a reader can distinguish applicable rationale
from uncertain history. Choosing a plausible occurrence is worse than refusing
to choose.

## Decision

Keep language-independent exact excerpts and capture up to three complete lines
immediately before and after the excerpt. Users choose the excerpt; koment
derives context and the last confirmed line from the source. Fewer context
lines are stored at a file boundary.

Resolution is deterministic:

1. A missing file is `orphaned`.
2. File scope is `ok` while the file exists.
3. No exact excerpt is `drifted`.
4. One exact excerpt resolves.
5. Several exact excerpts are filtered by captured context.
6. Exactly one contextual candidate resolves; every other multiple-candidate
   result is `ambiguous`.
7. A unique result at the confirmed line is `ok`; a unique result elsewhere is
   `moved`.

`ambiguous`, `drifted` and `orphaned` fail `koment check`. `last_seen_line`
describes movement and never selects a candidate. Every presentation includes
status, occurrence count and the same warning.

## Consequences

- Duplicating annotated code can break the check until the annotation is
  reanchored or given a more specific excerpt.
- Small movement remains a successful deterministic resolution.
- Context makes common duplicates resolvable without a language parser.
- Context is stored data that must be updated by reanchor, not hand-edited by a
  caller.
- The status model gains a fifth value and all consumers must handle it
  exhaustively.

## Alternatives rejected

- **Let the last line choose.** Fast and compatible, but line identity is brittle
  under insertion and contradicts the promise that line is only a hint.
- **Return the first occurrence with a warning.** The chosen location still
  looks authoritative to any consumer that loses or ignores the warning.
- **Require globally unique excerpts with no context.** Correct but hostile to
  common code shapes and generated repetition.
- **Parse language symbols with tree-sitter.** Stronger semantic identity, but it
  introduces grammars and language-specific behaviour before the deterministic
  core is proven.
- **Ask an LLM to choose.** It can produce a useful suggestion later, but a
  probabilistic answer cannot define ground truth for `koment check`.
