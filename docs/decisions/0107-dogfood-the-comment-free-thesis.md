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

The familiar way to record a local explanation is still to type a comment.
Forcing a user to remember a separate UI or command at that moment would make
koment least intuitive precisely where it should be most useful. At the same
time, editors and shells can always write arbitrary bytes, local Git hooks can
be bypassed and editor save hooks are not a security boundary.

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

Treat a completed explanatory comment as comment intent. The shared application
service can convert it into a normal annotation, durably writing the annotation
before removing the prose from source. Editor integrations call that operation
automatically when classification and anchoring are unambiguous, render the
result at the same place and present a diagnostic with a conversion action when
they are not. The comment's placement chooses a nearby code or file anchor; the
comment being removed cannot anchor its replacement. CLI and MCP expose the same
operation.

Keeping other prose inline requires an explicit policy acknowledgement. The
acknowledgement is a `why` annotation anchored to the exact comment with:

```yaml
policy:
  exception: inline-comment
  acknowledged: true
```

The CLI requires `--acknowledge-inline-comment`, MCP requires the corresponding
true boolean and an editor confirmation names the procedure being waived. The
annotation body explains why rename, extraction, a named type or constant,
restructuring and an ordinary annotation were insufficient. No inline ignore
directive is recognized. If the comment changes, its acknowledgement no longer
resolves and the policy check fails.

Define the enforceable guarantee as "cannot land" rather than "cannot type."
`koment comments check` fails on every non-intrinsic comment without an exact
acknowledgement. Local hooks and editor diagnostics provide fast feedback; the
required CI job on a protected branch is authoritative.

Migrate existing commentary deliberately after the reset record exists. Each
removal must leave code readable or move its rationale to an attributable
annotation. Do not bulk-delete comments merely to make a count reach zero.
Audit workflows, container files and Helm separately so schema directives and
generated documentation markers are not mistaken for ordinary commentary.

koment's own annotations must pass `koment check` in CI. An agent-created
annotation records agent authorship and never inherits a human identity
silently. ADR 0108 defines how repository-owned agent guidance, MCP
instructions, client hooks and the authoritative CI policy compose without
pretending any prompt can control an arbitrary process.

## Consequences

- Code review focuses on names and structure instead of narration.
- The project becomes a demanding real-world fixture for its own storage,
  retrieval and drift checks.
- Comment-policy tooling requires language awareness for Go and explicit policy
  for other file types.
- Users can begin with the comment gesture they already know and still produce
  an external, attributable annotation.
- A deliberately retained inline comment is reviewable data rather than an
  unstructured exception or suppression token.
- Temporary or partial editor failures may leave both comment and annotation,
  but the record-first order does not lose rationale.
- Some exported API documentation remains inline by design; the goal is no
  hidden implementation rationale, not an empty token count.
- Moving rationale creates more annotations and exercises human and agent
  authoring before the approved design can be called complete.

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
- **Prevent comment bytes from entering an editor buffer.** Editor APIs are not
  universal mediation points, direct file writes remain possible and aggressive
  edit reversal would be hostile to temporary commented-out code.
- **Silently convert every comment token.** Directives, public documentation and
  disabled code are not rationale. Ambiguous classification must remain visible
  and fail loudly.
- **Allow an inline suppression directive.** It is easy to add, hard to audit
  and creates another comment whose only purpose is bypassing the comment rule.
