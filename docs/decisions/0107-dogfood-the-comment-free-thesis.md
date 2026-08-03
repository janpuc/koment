# 0107 — Enforce the comment-free thesis on koment itself

Date: 2026-08-03
Status: Accepted

## Context

koment claims code becomes easier to read and edit when rationale leaves inline
comments. The v0.2 Go implementation, workflows, Dockerfile and chart still
contain explanatory comments, including rationale already duplicated in ADRs
or annotations. A project that needs those comments to explain itself has not
proved its own value.

A blind comment ban is also wrong. Toolchain directives affect compilation,
deprecation markers are part of API migration, an upstream link can be the only
precise explanation of external behaviour and public API documentation serves a
different audience from local rationale.

## Decision

Apply this order before writing an inline comment:

1. Rename the thing.
2. Extract a function whose name states the intent.
3. Introduce a named type or constant.
4. Restructure the control flow or data model.
5. Record local rationale as a koment annotation or structural rationale as an
   ADR.

Allow only toolchain directives, necessary upstream links, deprecation markers
and genuine public API documentation. Enforce the Go rule with an AST-aware CI
check that distinguishes documentation and directives from implementation
commentary.

Migrate existing commentary deliberately after record v2 exists. Each removal
must leave code readable or move its rationale to an attributable annotation.
Do not bulk-delete comments merely to make a count reach zero. Audit workflows,
container files and Helm separately so schema directives and generated
documentation markers are not mistaken for ordinary commentary.

koment's own annotations must pass `koment check` in CI. An agent-created
annotation records agent authorship and never inherits a human identity
silently.

## Consequences

- Code review focuses on names and structure instead of narration.
- The project becomes a demanding real-world fixture for its own storage,
  retrieval and drift checks.
- Comment-policy tooling requires language awareness for Go and explicit policy
  for other file types.
- Some exported API documentation remains inline by design; the goal is no
  hidden implementation rationale, not an empty token count.
- Moving rationale creates more annotations and exercises human and agent
  authoring before vNext can be called complete.

## Alternatives rejected

- **Allow comments that explain why.** Conventional and often useful, but it
  bypasses the product, omits author trust and cannot be queried consistently by
  agents.
- **Ban every comment token.** Simple to measure, but breaks directives,
  deprecation tooling and legitimate public API documentation.
- **Delete all existing comments mechanically.** Produces a clean metric while
  discarding exactly the history koment is meant to preserve.
- **Rely on reviewer discipline without CI.** The rule will regress gradually,
  especially when contributors do not know the project's thesis.
