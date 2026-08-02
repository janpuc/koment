# 0015 — Record an explicit author identity

Date: 2026-08-02
Status: Accepted

## Context

Annotations are anonymous. `created` says when; nothing says who.

That was tolerable while the store was one person's notes to themselves and to
agents working in the same checkout. It stops being tolerable as soon as an
annotation is read by someone who was not there: *"the skew subtraction is
deliberate, it bit us in #412"* is worth much more when you can tell whether it
came from the person who fixed #412 or from an agent inferring a rationale.

`git config user.name` and `user.email` are the obvious default — they are
already configured, already how commits are attributed, and require no new
setup. They are not sufficient as the only source:

- An annotation created through a web UI has an authenticated session identity
  that may not match any local git config.
- An annotation created by an agent should say so, rather than silently
  inheriting the human's git identity and claiming to be them.
- `git config` is trivially set to anything; it asserts, it does not prove.

## Decision

Every annotation records an author, captured at creation:

```yaml
author:
  name: Jan Pucilowski
  email: janpuc@proton.me
  kind: human
  source: git-config
```

- `name`, `email` — display identity.
- `kind` — `human` or `agent`. An agent must set this; nothing infers it.
- `source` — where the identity came from: `git-config`, `session`, `explicit`.
  This is the honest part. It records how much the identity is worth, rather
  than presenting every identity as equally trustworthy.

Two fields are reserved and unused for now, defined here so their meaning is
fixed before something needs them:

- `account` — a provider-scoped account id, once there is a provider.
- `verified` — the mechanism that proved the identity, if any. Absent means
  unverified, which is the honest default for `git-config`.

Defaults, in order: an explicit `--author` flag, then an authenticated session,
then `git config`, then failure. koment does not write an annotation with no
author.

**The identity is a claim, not a proof, unless `verified` says otherwise.** The
UI must present it that way. An unverified `git-config` identity is worth
exactly as much as the honesty of whoever ran the command, and rendering it
identically to an authenticated one would be a lie of omission.

## Consequences

- Annotations become attributable, and agent-written ones become distinguishable
  from human ones — which matters most when an agent is deciding how much weight
  to give a note that another agent wrote.
- **Email addresses land in a committed file, in a repository that may be
  public.** This is the same exposure as a git commit, so it surprises nobody,
  but it is exposure. `--author` accepts a name with no email for anyone who
  wants less.
- One more required field, so `add` can now fail on a machine with no git
  identity configured. Failing is correct: an unattributed annotation is a
  weaker record, and the fix is one `git config` away.
- Existing annotations have no author. They are read as `unknown`, not
  backfilled. Inventing an author for a record that never had one would be
  fabricating provenance.
- `verified` being reserved-and-empty means the honest answer today is that no
  identity is proven. That is a true statement about the current system rather
  than a gap in this ADR.

## Alternatives rejected

- **Derive the author from git blame of the annotation file.** No new fields,
  and it is cryptographically as strong as the commit signature. Rejected
  because it attributes the *commit*, not the *annotation*: a rebase, a squash,
  a `git add -p` by a colleague, or a bulk `reanchor` all reattribute
  annotations someone else wrote.
- **git config only, with no `source` or `kind`.** Simplest, and what most tools
  do. Rejected because it cannot distinguish an agent from the human whose
  machine it ran on, which is precisely the distinction a multi-agent codebase
  needs.
- **Require authenticated identity for every annotation.** Strongest guarantee.
  Rejected as premature: it forces an identity provider on a tool that currently
  needs no server, and would block `koment add` on a laptop with no network.
- **Store no author; let git history answer it.** The status quo. Rejected for
  the reason above — history answers who committed the file, which is a
  different question, and the answer degrades with every history rewrite.
