# 0001 — Record architecture decisions

Date: 2026-07-31
Status: Accepted

## Context

koment exists because rationale that lives in code comments rots. That argument
applies to koment's own rationale. If we do not record decisions somewhere
durable and reviewable, a future agent will refactor away a constraint it cannot
see — the exact failure this project was built to prevent.

## Decision

Every non-obvious decision is recorded as an ADR in `docs/decisions/`, numbered
sequentially, in the Nygard format: Context, Decision, Consequences, and
explicitly **Alternatives rejected**.

ADRs are immutable once accepted. A change of mind produces a new ADR that
supersedes the old one; the old one is marked `Superseded by NNNN` and kept.

The "Alternatives rejected" section is mandatory. An ADR that records only what
was chosen has thrown away its most valuable half — the next agent needs to know
which roads were already walked.

## Consequences

- Structural changes cost a document. That friction is intended.
- The decision history is readable by any agent with filesystem access, with no
  tooling and no server.
- ADRs and koment annotations have distinct jobs: ADRs hold project-wide
  decisions, koment holds knowledge bound to a specific place in the code. When
  in doubt, prefer an ADR — it has no anchoring problem.

## Alternatives rejected

- **A single CHANGELOG or DECISIONS.md.** Grows without bound, merge-conflicts
  constantly, and gives no stable identifier to reference or supersede.
- **Decisions in commit messages.** Real, but unfindable: nobody greps six
  months of `git log` before a refactor, and squash-merges destroy them.
- **A wiki or external doc site.** Drifts from the code, needs separate auth,
  and is invisible to an agent working from a checkout.
