# Agent rules for koment

<!-- koment:managed-start -->
# KOMENT PROCEDURE — MANDATORY

This file is a managed contract. The repository enforces it with
client hooks and a required CI status. You MUST follow every rule.
Partial compliance is a bug.

## Before any edit or write to an existing file

- You MUST invoke `koment_get`("<repository-relative path>") via MCP
  and treat the returned annotations as the authoritative context for
  the file. An annotation whose status is `ambiguous`, `drifted`
  or `orphaned` is history, not current fact; surface it to the human
  before continuing.
- You MUST invoke `koment_search`("<topic>") before any non-obvious
  structural decision that another file may already explain.

## Adding or changing a comment is FORBIDDEN

- You MUST NOT write an ordinary inline comment in source. The
  repository classifies every comment group and rejects ordinary ones
  on the protected branch. The only exceptions are the intrinsic
  classes enabled in `.koment/policy.yaml` (toolchain directives,
  generated markers, upstream links, `// Deprecated:`, godoc on
  exported identifiers) and any additional pattern declared under
  `spec.comments.allowedAnnotations`.
- Before keeping a comment, you MUST attempt in order: rename the
  thing, extract a function whose name is the comment you were about
  to write, introduce a named type or constant, restructure so the
  invariant is obvious from control flow. If the rationale still needs
  saying, call `koment_add` bound to the code with `--excerpt`
  and record yourself honestly as an agent.
- If a comment already exists in source, you MUST call
  `koment_convert_comment` first to record it as an annotation,
  then remove the comment from source.
- Keeping an inline comment requires `koment_acknowledge_comment`
  with `acknowledge_inline_comment: true` and a human-readable body.
  The acknowledgement is auditable.

## Before you stop

- You MUST run `koment check`, `koment comments check` and
  `koment agents check`. You MUST NOT report success while any
  fails.

A back-compatibility claim needs evidence: a migration path the binary performs, or an ADR naming the version the old shape was cut off at. Without either, the change is breaking and its commit subject says so with `feat!:`.
<!-- koment:managed-end -->

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
- `koment_repositories()` — repository names when the server has more than one

Without MCP, use the installed CLI: `koment show <file>`.

An annotation whose status is `ambiguous`, `drifted` or `orphaned` describes
code that cannot be resolved reliably. Read it as history and say so; never act
on it as if it were current.

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

- **Never state a library's API, flags or defaults from memory.** Check
  `pkg.go.dev`, the vendored source in the module cache, or this project's own
  code. Model memory of a library is a snapshot of some version, and it is not
  this one.
- Do not report something works without running it.
- If you could not verify a claim, say so explicitly in your summary.
- Quote real output when reporting results. Paraphrased output is not evidence.
- Toolchain versions come from `go.mod` and `.mise/config.toml`, not from
  assumption. Those files must carry the same Go version.
- Run repository tasks through `mise run`. `.mise/config.toml` and its lock
  file define the exact tools CI uses; `mise tasks` is the command reference.

## 5. Fail loudly

A tool that silently serves a stale annotation is worse than one that crashes.

- Never swallow an error to keep going.
- `_ = someCall()` is only for genuinely fire-and-forget calls, and the reason
  must be obvious from context. It is a deliberate discard, never a way to quiet
  a linter.
- Never return a partial result that looks complete.
- When an anchor cannot be resolved, say so in the output — do not omit it.
- Prefer a non-zero exit and a clear message over a best guess.

Logging is `log/slog` when logging is needed. Most of koment writes to the
`io.Writer` it was handed instead, which is what makes the commands testable —
do not reach for a global logger.

## 6. Tests are part of the change

- Every anchoring rule gets a test with a real before/after file pair.
- Drift detection gets tests for each status the model can produce.
- Run `mise run test`; it includes the race detector and coverage.
- Run the tests. Paste the output in your summary.
- A change to parsing or anchoring without a test is not finished.

## 7. Small, reviewable changes

One concern per commit. If the diff needs a section-by-section walkthrough to
review, split it.

**Do not refactor or "improve" adjacent code.** If you notice something worth
fixing while you are in a file, say so in your summary and leave it. A change
that also tidies three unrelated things is a change nobody can review, and it
buries the part that mattered.

Configuration is flags with a `KOMENT_` environment fallback, wired through
`internal/config`. Anything a person might reasonably want to change should be
settable both ways; adding a flag gets the environment variable for free.

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

This repository is koment's own first user (ADR 0107). When you find yourself
about to write a comment explaining *why*, and rules 1–4 above have not
dissolved it, the rationale belongs in one of two places:

- Project-wide, or about a structure rather than a place → an ADR.
- Bound to a specific place in the code → a koment annotation.

```
koment add <file> --excerpt '<verbatim snippet>' --agent \
    --kind gotcha --body -
```

`--body -` reads from stdin, which is easier than quoting prose in a shell.
Kinds are `why`, `gotcha`, `invariant`, `anti-pattern`.

Run `koment check` before you finish. It exits non-zero if any
annotation no longer resolves, including ones you invalidated by editing the
code they were anchored to. Fix the annotation or fix the anchor — do not delete
the annotation to make the check pass.

## 13. Working as an agent

You are trusted with a lot here, so be explicit about what you did.

- Record yourself honestly: `koment add --agent` sets the author kind so a
  reader can weigh an agent-written annotation differently from a human one.
  Never let an agent-written annotation inherit a human's git identity silently.
- Do not write pull request descriptions or commit messages that claim more than
  you verified. "Tests pass" means you ran them and can quote the output.
- If an instruction here conflicts with what you were asked to do, say so rather
  than quietly picking one. The conflict is usually the interesting part.

## 14. Releases follow the written procedure exactly

Cutting a release is the one task in this repository with consequences you
cannot take back. Marketplace and registry versions are permanent — a version
number cannot be reused, replaced or withdrawn once published.

**[docs/releasing.md](docs/releasing.md) is mandatory and authoritative. Read it
in full before touching a release, and follow its steps in order.**

The parts that are not negotiable:

- **Merging a release pull request and anything that publishes needs explicit
  human approval in the conversation first.** Preparing and verifying a release
  is yours; making it public is not.
- Never publish an artifact by hand, outside the release workflow.
- Never hand-edit a version: `release-please` owns every file that carries one,
  and `packaging` fails the build when they disagree.
- Never merge a release pull request whose checks did not run. Those pull
  requests are created by `GITHUB_TOKEN`, so their checks sit at
  `action_required` until approved; an unapproved run is not a passing run.
- Never delete or re-point a tag to redo a release. Cut the next patch instead.
- Never report a release as succeeded without verifying its published assets and
  quoting the output.

## 15. Reporting

When you finish, state plainly:

- what you changed
- what you verified, and the command output that proves it
- what you did **not** verify
- what you got wrong along the way

Do not oversell. A correction is more useful than a confident wrong claim.

### CI is part of "verified" for any pull request

Local `mise run` is the floor, not the ceiling. A pull request is
not done while any required GitHub Actions check on it is failing,
pending, or missing. Before reporting a PR as ready:

1. Inspect the check status (`gh pr checks <n>` or the Actions UI).
   Every required status must read `pass`. Pending, queued, failed,
   cancelled, or skipped checks are not done.
2. Quote the check names and their outcomes in the report. The reader
   should not have to open a second tab to see what passed.
3. If a check failed for a reason inside this repository's code or
   configuration, fix it and push. Do not report done until the rerun
   is green.
4. If a check failed for a reason outside the repository — a GitHub
   Actions infrastructure outage, a runner-image regression, a flaky
   network call to a third-party service — say so explicitly. A
   transient infrastructure failure does not certify the change;
   re-run the failed jobs and quote the new outcome. Naming the flake
   is the honest move; calling a rerun "verification" without saying
   the first attempt failed is not.
5. A CI configuration that uses `GITHUB_TOKEN` to open its own pull
   request leaves its checks at `action_required` until a human
   approves them. An unapproved run is not a passing run. State the
   approval state if it matters for what you are claiming.

The same rule applies to a release candidate (see §14): quoting
local `mise run test` output for a release is necessary but not
sufficient. The release pull request's required checks must be green
on a human-approved run before the release can be reported done.
