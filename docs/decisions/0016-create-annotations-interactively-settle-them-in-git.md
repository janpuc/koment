# 0016 — Create annotations interactively, settle them in git

Date: 2026-08-02
Status: Accepted

Answers the open question of *when* an annotation comes into existence.

## Context

Three models were on the table.

**1. Annotations are committed with the code.** What koment does today. Git is
the only store; an annotation exists when it is committed. Strong history,
offline by default, reviewable in the same pull request as the change that
motivated it, attribution follows the commit. The cost is latency and friction:
writing a note means staging and committing a file, which is a poor fit for the
moment a reader actually has the thought.

**2. The deployment stores annotations immediately.** A server owns them. No
commit required, good interactive experience, and the application manages
history. The cost is that annotations stop being in the repository — they need
their own auth, their own backup, their own history, and a clone of the code no
longer contains the reasoning about it. ADR 0004 rejected exactly this shape
after measuring what a consolidating store does to verbatim records, and ADR
0002 chose the repository so that annotation and code move together.

**3. Hybrid.** Created interactively, stamped with the current commit, later
materialised into the repository.

The hybrid is appealing and is also where this design can quietly go wrong. "The
application manages comment history, git manages file history" describes two
writable stores holding the same records, which is two sources of truth. The
question that decides whether the hybrid is coherent is not *"can we sync?"* but
*"which one is right when they disagree?"*

## Decision

Adopt the hybrid, with one asymmetry that keeps it from becoming two masters:

**The repository is the record. The deployment store is an outbox.**

- An annotation created interactively is written to the deployment store
  immediately, stamped with its git context (ADR 0014) and author (ADR 0015),
  and is **pending**.
- Pending annotations are served, clearly marked as not yet in the repository.
- Materialising writes them into `.koment/annotations/` and commits. From that
  moment the annotation is **settled** and the repository is authoritative.
- A settled annotation is never written back to the deployment store as
  editable state. The store keeps no second copy to drift.
- On conflict, git wins. Always. The store is a queue of things not yet in git,
  not a mirror of things that are.

The ULID is minted at creation and survives materialisation, so a pending and a
settled annotation are the same annotation with the same identity.

This preserves the invariants the earlier ADRs were bought with:

- **A clone still contains all settled reasoning** (ADR 0002).
- **The read path still requires no server** — the CLI and the MCP server read
  `.koment/` and never consult the deployment (ADR 0004). A deployment that
  vanishes costs its outbox, not the record.
- **Nothing paraphrases, merges or expires an annotation** (ADR 0004). The
  outbox stores verbatim and forgets nothing on its own.

## Consequences

- The good interactive experience is available without giving up git as the
  record, which was the point.
- **An unmaterialised annotation is durable only as far as the deployment's
  storage is.** It is not in anyone's clone. The UI must say so plainly rather
  than showing pending and settled identically, and materialising should be
  easy and prompted, not a chore someone forgets.
- Two states to reason about everywhere: `pending` and `settled`. That is real
  added complexity, and it is the price of not having two masters.
- Materialising produces a commit, so it needs a writable checkout and a
  committer identity. In a hosted deployment that means a bot identity and push
  rights — a genuine operational dependency, and the reason this cannot be an
  afterthought.
- **Interactive creation is a write path, and ADR 0011 attached a condition to
  itself: the no-authentication posture must be revisited before any write path
  ships.** That condition is now live. Nothing in this ADR may be exposed on a
  network listener until authentication exists. The read-only server may keep
  shipping unauthenticated; the moment it can create an annotation, it may not.

## Alternatives rejected

- **Option 1, commit-only.** Simplest, and it is what exists. Rejected only
  because the friction is real and suppresses annotations that should have been
  written — the failure is silent and looks like "nobody had anything to say".
  It remains the right model for the CLI, which keeps working exactly as it
  does.
- **Option 2, deployment-owned.** Best interactive experience and the cleanest
  implementation, because there is one store and no materialisation. Rejected
  because it takes the reasoning out of the repository, which is the single
  decision this project is built on. A clone that does not contain its own
  rationale is the problem koment exists to solve, not a shape it may adopt.
- **A symmetric hybrid, syncing both ways.** The obvious reading of "hybrid",
  and the reason to write this ADR rather than nod at the idea. Two writable
  copies of the same record need conflict resolution, and the only honest
  resolutions are "last write wins" — which loses annotations — or a merge UI,
  which is a product in its own right. Making the store an outbox removes the
  conflict instead of managing it.
- **Materialise automatically on every write.** No pending state to explain, and
  it collapses the model back to option 1 with nicer ergonomics. Rejected
  because a commit per annotation is noisy, needs push rights on the interactive
  path, and fails badly offline or when the branch is protected — which it is
  here.
