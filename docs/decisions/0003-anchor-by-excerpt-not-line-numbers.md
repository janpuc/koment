# 0003 — Anchor by excerpt, not line numbers; defer symbol scope

Date: 2026-07-31
Status: Accepted

## Context

Anchoring is *the* hard problem in out-of-band annotation, and has been since at
least Microsoft Research's 2001 work on robustly anchoring annotations using
keywords. Every scheme trades precision against survivability:

| scheme | survives | breaks on |
|---|---|---|
| line number | nothing | the next edit above it |
| excerpt text | reformatting elsewhere, moves | editing the excerpt |
| symbol name | moves, reformatting, edits inside the body | rename |
| content hash | nothing | any edit |

## Decision

v1 supports two scopes:

- `file` — the annotation is about the file as a whole. Cannot drift except on
  rename or delete.
- `excerpt` — a verbatim snippet stored with its SHA-256. Resolution searches the
  current file for the snippet.

Line numbers are never stored as an anchor. Symbol scope is deferred.

An excerpt that no longer matches is reported as `drifted` — a failure, not a
warning. The annotated code changed and nobody revisited the annotation, which
is exactly how comments rot.

## Consequences

- No language parser, no grammar, no dependency. Resolution is a substring
  search and a hash comparison, and behaves identically in every language and
  file format — including the YAML manifests that motivated this project.
- Editing annotated code forces you to re-confirm the annotation. This is
  friction by design, and the core value: a stale annotation is worse than none.
- Refactor-heavy work will produce drift noise. If that proves intolerable in
  real use, that is the evidence needed to justify symbol scope — not a
  hypothetical.

## Alternatives rejected

- **Line numbers.** Rot on the very next edit. Non-negotiable.
- **Symbol scope in v1** (via tree-sitter or LSP). Genuinely better anchoring:
  survives moves, reformatting and edits within a body. Rejected for now because
  it costs a large dependency plus a grammar per language, and would not cover
  the YAML and shell that motivated this project. Revisit once v1 is in real
  use, with drift statistics to justify it.
- **Whole-file content hash.** Every edit anywhere invalidates every annotation
  in the file. Maximum noise, minimum signal.
- **LLM semantic re-anchoring up front** (Codetations / Magic Markup). The
  interesting approach, and the eventual goal. Rejected as a starting point:
  without a deterministic layer there is no ground truth to evaluate re-anchoring
  against, and a system that silently re-attaches annotations to the wrong place
  is worse than one that reports drift.
