# 0117 — Relicense to AGPL-3.0-or-later with commercial dual licensing

Date: 2026-08-05
Status: Accepted

## Context

koment had been MIT-licensed since its first release. Six releases (v0.3.1–v0.6.0)
are already public and remain MIT in perpetuity; anyone may fork from the last
MIT tag forever. That history is permanent and this ADR does not pretend to
change it.

Two facts make the license a now-decision rather than a later one:

1. The repository has a single human author. Relicensing now costs nothing.
   Once an external PR is merged under MIT without a CLA, every contributor's
   consent is needed before any non-permissive relicensing — and dual
   licensing becomes a slow legal exercise instead of a flag flip.
2. The served tier (`koment serve`) is a network service. MIT lets any
   organisation deploy a modified served tier behind a firewall without ever
   giving anything back, and the product's most enterprise-leaning component
   is exactly the one the permissive license fails to defend.

The stated goals driving the decision:

- the project must stay OSI-approved Open Source (verified by
  `https://opensource.org/licenses/`);
- the license must create a defensible sales motion for organisations that
  cannot use the project under its public terms;
- the product must remain attractive to solo developers and small teams who
  drive adoption, which means the local and published tiers must not gain
  surprising new obligations.

For a *tool*, copyleft does not infect the user's code. Annotating a project
with an AGPL binary does not make the annotated project AGPL; `.koment/`
annotation YAML is data, not a derivative work. That is the same legal class
as compiling a program with GCC. The FAQ in `README.md` carries the same
point for readers who do not already know it.

## Decision

From the next release, the repository is licensed under **GNU Affero General
Public License v3.0-or-later**. A commercial licence is offered on request for
organisations whose policy excludes AGPL or who want warranty or
indemnification; contact details live in `README.md`.

The change applies only to source published from this point forward. Releases
≤ v0.6.0 retain their original MIT terms and may be forked from the last MIT
tag indefinitely.

Every external contribution must be covered by a Contributor Licence Agreement
before the first merged PR. The CLA grants the project the right to relicense
contributed code under both AGPL and the commercial licence. A lightweight CLA
Assistant GitHub Action backs the workflow.

The "koment" name and logo are **not** licensed under the AGPL grant and are
governed separately by `TRADEMARK.md`. A fork must rename itself; the
marketplace listings, MCP Registry entry and verification labels are bound to
the trademark holder.

## Consequences

Easier:

- A standing commercial-licensing offer can be quoted to any organisation that
  asks the question that AGPL bans already trigger.
- The project retains the OSI "Open Source" label and the right to use the
  marks associated with it.
- The whole repository ships under one licence; there is no per-tier license
  split to maintain.
- Released work ≤ v0.6.0 remains forkable, which keeps the historical record
  honest and prevents bad-faith accusations of a rug pull on existing
  consumers.

Harder:

- Some enterprises reject AGPL outright through blanket policy (Google and
  others). Those consumers must either use the commercial licence, run a
  pre-AGPL release, or not adopt. Adoption ceiling is lower than MIT.
- The CLA is an extra step for every external contributor. A friendly
  CLA Assistant bot is the smallest workable overhead; it still slows the
  first PR of a new contributor.
- The pre-AGPL fork is permanent. A hostile third party may distribute
  v0.6.0 in perpetuity and call itself by any name except "koment". The
  trademark is the lever against the name collision, not the code.

Committed to:

- A standing offer to quote commercial licences to any organisation that
  asks, and a non-zero response time on that offer.
- The CLA being the boundary that preserves dual licensing for contributed
  code. Merge without a CLA is rejected at the policy gate, not at
  code review.
- `TRADEMARK.md` being kept current and a EUIPO (and ideally USPTO)
  registration being pursued as a separate workstream outside this
  repository.

## Alternatives rejected

- **Stay MIT.** Concedes the entire free-rider concern. The served tier can
  be modified and operated as a network service with no obligation to
  publish source. No commercial-licensing motion exists at all. Only
  upside was maximum adoption ceiling; every other dimension loses.
- **Apache-2.0.** Adds the patent grant MIT lacks, which is genuinely
  useful for a developer tool. Still permissive on the network-service
  question; still concedes the free-rider concern. Considered only as the
  fallback if AGPL were not viable for some unforeseen reason.
- **GPL-3.0 without the network clause.** Closes the local-copy loophole
  but leaves the served tier — the product's most enterprise-relevant
  surface — entirely exposed. Worse than AGPL at exactly the boundary that
  matters.
- **BSL 1.1 with a 3-year change date.** Strongest competitive protection
  and no enterprise-policy friction, but not OSI-approved. Fails the
  stated OSI hard requirement. The HashiCorp→OpenTofu split is also a
  real fork precedent once a project reaches critical mass; for a
  pre-adoption project the bigger risk is adoption chill, but the OSI
  label alone disqualifies it.
- **Fair Source License (FSL) / Elastic License v2 (ELv2).** Source-
  available, never open. ELv2 in particular lacks a change-date path to a
  truly open licence, which is a permanent rather than temporary
  reduction in the project's openness posture.
- **SSPL.** Server Side Public License. OSI has rejected it and major
  Linux distributions refuse to ship it. Distribution-toxic and out of
  scope for a project that wants to be installable everywhere.
- **Per-tier license split** (MIT for the local CLI, AGPL for the served
  tier, etc.). Premature complexity with no adoption data to justify the
  engineering and review cost of maintaining tier-boundary enforcement
  in the build. Revisit only after served-tier adoption data exists.
- **Patent-only defence against clones.** Expensive, slow, hostile to the
  project's OSS posture, and does nothing the trademark + velocity
  combination does not already do at far lower cost.
