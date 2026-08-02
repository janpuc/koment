# Agent rules for koment

You are working on a tool whose entire purpose is to make code understandable
without comments. If this codebase needs comments to be understood, the project
has failed its own thesis. Dogfood it.

**New here? Read [docs/bootstrap.md](docs/bootstrap.md) first** — what this is,
how the data model works, how to run and test it, and the rules below in
context.

Read `DESIGN.md` before writing code. Read `docs/decisions/` before changing
anything structural.

**Before you edit any file in this repository, read its annotations.** They hold
the reasoning that is deliberately not in the comments. `.mcp.json` is committed,
so an MCP-capable client already has the tools:

- `koment_get(file)` — annotations for the file you are about to touch
- `koment_search(query)` — find recorded rationale by topic

Without MCP, use the CLI: `go run ./cmd/koment show <file>`.

An annotation whose status is `drifted` or `orphaned` describes code that has
since changed. Read it as history and say so; never act on it as if it were
current.

---

## 1. Readable code is a hard rule

Comments are a **last resort**, not a default. Before writing one, try in order:

1. Rename the thing. Most comments exist because a name is bad.
2. Extract a function whose name is the comment you were about to write.
3. Introduce a named type or constant instead of a bare value.
4. Restructure so the invariant is obvious from control flow.

Only when all four fail does a comment earn its place — and then it explains
**why**, never **what**.

```go
// BAD - narrates the code
// loop over annotations and check if the anchor still matches
for _, a := range annotations { ... }

// BAD - a comment doing a function's job
// resolve the symbol, then compare the hash, then mark drifted
...30 lines...

// GOOD - the name is the comment
for _, a := range annotations {
    if a.Anchor.HasDrifted(file) { ... }
}
```

**Rationale that would have been a comment goes in an ADR.** That is the whole
point of this project. `docs/decisions/`.

Exceptions, and these are the only ones:

- A `//go:` directive or equivalent toolchain pragma.
- A link to an upstream issue/spec that explains non-obvious external behaviour.
- A `Deprecated:` marker.
- Godoc on exported identifiers — this is API documentation, not commentary.

## 2. Every non-obvious decision gets an ADR

If a future reader could reasonably ask "why is it like this?", write an ADR.
Use `docs/decisions/NNNN-kebab-title.md`, next free number, and follow the
existing format exactly.

An ADR must record the **alternatives you rejected and why**. An ADR that only
states what was chosen is worthless — the value is in the road not taken.

Supersede rather than edit: mark the old one `Superseded by NNNN` and write a
new one. The history is the product.

## 3. Design before code

For anything beyond a bugfix: write or update `DESIGN.md` first, get it agreed,
then implement. Do not open a large diff that also invents the design.

## 4. Verify, never assume

This project exists because stale information is worse than no information.
Hold yourself to the same bar.

- Do not claim a library, flag or API exists without checking it.
- Do not report something works without running it.
- If you could not verify a claim, say so explicitly in your summary.
- Quote real output when reporting results. Paraphrased output is not evidence.

## 5. Fail loudly

A tool that silently serves a stale annotation is worse than one that crashes.

- Never swallow an error to keep going.
- Never return a partial result that looks complete.
- When an anchor cannot be resolved, say so in the output — do not omit it.
- Prefer a non-zero exit and a clear message over a best guess.

## 6. Tests are part of the change

- Every anchoring rule gets a test with a real before/after file pair.
- Drift detection gets tests for each status the model can produce.
- Run the tests. Paste the output in your summary.
- A change to parsing or anchoring without a test is not finished.

## 7. Small, reviewable changes

One concern per commit. If the diff needs a section-by-section walkthrough to
review, split it.

## 8. Git discipline

- **Never commit or push unless explicitly asked.** Leave work in the tree.
- Never `git add -A` blindly. Stage deliberately.
- Never rewrite published history.
- Conventional commit subjects: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`,
  `chore:`.

## 9. Naming

- Names say what a thing *is*, not how it is implemented. `Anchor`, not
  `AnchorStruct`. `Resolve`, not `DoResolve`.
- No abbreviations except the universally understood (`id`, `url`, `sha`).
- Booleans read as assertions: `HasDrifted`, `IsOrphaned`.
- A function that returns a decision is named for the decision, not the check.

## 10. Dependencies

Every new dependency needs an ADR. The bar is high: standard library first, and
a small well-understood module over a large framework. This tool has to be
trustworthy and long-lived; a dependency is a liability you cannot easily
retire.

## 11. Scope discipline

Do not build the LLM re-anchoring layer, the web UI, the IDE plugin, or the
"AI suggests annotations" feature until the deterministic core is finished and
in real use. See `DESIGN.md` for what "finished" means. Adding intelligence to
a system whose fundamentals are unproven is how this becomes another abandoned
research prototype.

## 12. Record rationale as an annotation

This repository is koment's own first user (ADR 0010). When you find yourself
about to write a comment explaining *why*, and rules 1–4 above have not
dissolved it, the rationale belongs in one of two places:

- Project-wide, or about a structure rather than a place → an ADR.
- Bound to a specific place in the code → a koment annotation.

```
go run ./cmd/koment add <file> --excerpt '<verbatim snippet>' \
    --kind gotcha --body -
```

`--body -` reads from stdin, which is easier than quoting prose in a shell.
Kinds are `why`, `gotcha`, `invariant`, `anti-pattern`.

Run `go run ./cmd/koment check` before you finish. It exits non-zero if any
annotation no longer resolves, including ones you invalidated by editing the
code they were anchored to. Fix the annotation or fix the anchor — do not delete
the annotation to make the check pass.

## 13. Reporting

When you finish, state plainly:

- what you changed
- what you verified, and the command output that proves it
- what you did **not** verify
- what you got wrong along the way

Do not oversell. A correction is more useful than a confident wrong claim.
