# 0010 — Dogfood koment on koment, and make the annotations reachable by default

Date: 2026-07-31
Status: Accepted

## Context

DESIGN.md lists "koment's own `.koment/` holds real annotations about koment" as
a criterion for v1, with the note that it is not decoration. That is a good
instinct stated as a checklist item, which means it can be satisfied once and
then quietly rot. It needs to be a decision with a mechanism.

There is a second, larger gap behind it. Annotations that exist but that nobody
reads are worth nothing. AGENTS.md tells an agent to read `DESIGN.md` and
`docs/decisions/`; it says nothing about the annotations, and no agent will
invent `koment_get` on its own. A store that requires the reader to already know
it exists has the same failure mode as a wiki.

This repository is also the only place where koment's own rules apply to koment.
AGENTS.md forbids comments that explain *why*. Without somewhere for that
rationale to go, the rule produces code with the reasoning deleted rather than
code with the reasoning relocated.

## Decision

koment annotates itself, and the annotations are reachable without anyone being
told how.

1. Rationale that would have been a `why` comment in koment's own source is
   recorded as a koment annotation, not deleted and not left as a comment. The
   comments removed when this ADR was written became the first nine annotations.
2. `.mcp.json` is committed at the repository root, configuring `koment mcp`
   over stdio. Any MCP-capable agent opening this checkout discovers
   `koment_get` and `koment_search` with no setup.
3. AGENTS.md instructs agents to consult annotations for a file before editing
   it, and to record rationale as an annotation rather than a comment.
4. `koment check` runs in CI. Drift fails the build, exactly as `drifted` being
   a failure rather than a warning requires.

Point 2 is what makes the rest work. Points 1 and 3 are policy and can be
forgotten; point 2 changes what an agent sees by default.

## Consequences

- The tool is exercised on a real codebase from the first commit, so anchoring
  problems surface here before they surface for anyone else. Writing these nine
  annotations is what exposed that yaml.v3 never wraps long scalars (ADR 0006).
- Every future change to koment's own source can drift its own annotations, and
  CI will say so. That friction is the product working, not the build being
  flaky.
- `.mcp.json` bootstraps by running `go run ./cmd/koment mcp`, which needs a Go
  toolchain but no build step or install. A contributor with a prebuilt binary
  can point the config at it instead.
- Committing `.mcp.json` means every agent in this repository spawns a koment
  process. The cost is two tool schemas of context per request, which ADR 0005
  already budgeted for.
- An agent that ignores AGENTS.md still gets the tools, and an agent that reads
  it still has to choose to call them. This raises the floor; it does not
  guarantee anything.

## Alternatives rejected

- **Satisfy the criterion once and move on.** What DESIGN.md literally asked
  for. Rejected because a checklist item with no mechanism decays: nothing would
  notice annotations going stale, and nothing would tell the next agent they
  exist.
- **Document the MCP setup in the README and let people wire it up.** Zero new
  files. Rejected because it makes access opt-in for exactly the readers least
  likely to opt in — a fresh agent on its first task in the repository.
- **A generated AGENTS.md fragment listing every annotation.** Already rejected
  in ADR 0005 for whole-repo context and staleness; it is no better here, and it
  would duplicate the store into a file that drifts from it.
- **A `PreToolUse` hook that injects annotations on Read/Edit.** The highest
  fidelity option, and the one the memini deployment already proves works. Left
  for later, per ADR 0005: it is one implementation per client, and it should be
  added on top of a working MCP path rather than instead of one.
- **Annotating exhaustively.** Tempting as a demonstration. Rejected because an
  annotation on something self-evident is noise that trains readers to skip the
  ones that matter, and every annotation is a maintenance liability that can
  drift.
