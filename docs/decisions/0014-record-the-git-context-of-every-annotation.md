# 0014 — Record the git context of every annotation

Date: 2026-08-02
Status: Accepted

Extends ADR 0003 and ADR 0009. Neither is reversed; the distinction that keeps
all three true is stated below.

## Context

An annotation currently records what it is about (`excerpt`), how to find it
(`excerpt_sha256`, `last_seen_line`) and when it was written (`created`). It
records nothing about the state of the world at that moment.

That is enough to answer *"does this still apply?"* and nothing else. It cannot
answer the questions a reader actually has when `check` reports drift:

- What did this file look like when the annotation was written?
- What changed between then and now?
- Was the annotated code moved, rewritten, or deleted?

Today the honest answer is "read the annotation and guess". The information
needed to answer properly already exists in git; koment simply never wrote down
which commit it was looking at.

The obvious objection is ADR 0003, which says line numbers rot and are never
stored as an anchor, and ADR 0009, which admits `last_seen_line` only as a hint
that cannot affect resolution. Recording a line number looks like a reversal.

It is not, because **anchoring and history are different jobs**:

| | question | answers with | when it changes |
|---|---|---|---|
| anchor | does this still apply to the code as it is now? | excerpt search | every resolution |
| git context | what was true when this was written? | commit hash | never |

A line number is worthless as an anchor precisely because the file moves under
it. A line number *paired with a commit hash* is exact and permanent: at commit
`abc1234`, `internal/store/ulid.go` line 18 is a fact that cannot rot, because
that commit is immutable.

## Decision

Every annotation records the git context in which it was created:

```yaml
git:
  commit: 9f3c1a4d8e2b7c5a6f0d3e1b8c7a5f2d4e6b9c1a
  path: internal/store/ulid.go
  line: 18
  end_line: 18
```

- `commit` — full SHA. **The authoritative reference.** Reconstructing the file
  as it was means `git show <commit>:<path>`, not searching for the excerpt.
- `path` — the path *at that commit*, which may differ from the record's current
  `file` after a rename.
- `line`, `end_line` — the range at that commit. Historical fact, never an
  anchor.

The context is written once at creation and **never rewritten**. `reanchor`
updates the anchor — `excerpt`, `excerpt_sha256`, `last_seen_line`, `file` — and
leaves `git` alone. An annotation's history is not editable; only its aim is.

Resolution is unchanged. `anchor.Resolve` reads the excerpt and nothing else.
Deleting the whole `git` block must not change any status, and a test asserts
that.

The excerpt stays, and stays load-bearing. It is what makes resolution work
without a git binary, in a worktree with uncommitted changes, and in a directory
that is not a repository at all. The commit hash is the authority on *history*;
the excerpt is the authority on *applicability*. Neither substitutes for the
other.

## Consequences

- A drifted annotation becomes explicable rather than merely flagged: the tool
  can show the file as it was, diff it against now, and let a reader judge
  whether the reasoning survived.
- Following renames becomes possible, because `git log --follow` has a starting
  commit and path to work from.
- **koment gains an optional dependency on git being present and the commit
  being reachable.** Optional is the operative word: everything that works today
  keeps working with no git binary, a shallow clone, or a squashed history. Only
  the historical views degrade, and they degrade to "cannot reconstruct" rather
  than to a wrong answer.
- Annotations created outside a repository, or in a dirty worktree with no
  commit yet, have no context to record. The field is omitted; it is not faked.
- Records get bigger and slightly noisier to read by hand. Accepted.
- A rewritten history — rebase, squash, filter-branch — orphans the referenced
  commit. The annotation still resolves, because resolution never consulted it;
  the historical view reports the commit as unreachable rather than pretending.

## Alternatives rejected

- **Store nothing; derive history from `git log -S <excerpt>` on demand.** No
  new fields, and it works for annotations already written. Rejected because it
  is a guess dressed as a fact: pickaxe search finds *a* commit touching that
  text, not the commit the author was looking at, and it fails outright once the
  excerpt has drifted — which is exactly when the history is wanted.
- **Store the commit hash only, and derive the line by searching that commit.**
  Smaller. Rejected because the search can fail or be ambiguous at the historical
  commit too, and the line is free to record at creation when it is known
  exactly.
- **Make the commit hash the anchor, replacing the excerpt.** The request that
  prompted this ADR leans this way. Rejected: an anchor must answer "does this
  still apply *now*", and a commit hash is by construction about the past. It
  would also make resolution require git, breaking the dirty-worktree and
  no-git cases that excerpt matching handles today.
- **Store a patch or the full file content at creation.** Perfect fidelity with
  no git dependency at all. Rejected as enormous: the store would grow without
  bound and would duplicate, badly, the thing git already does well.
