# 0017 — Make the repository a first-class scope

Date: 2026-08-02
Status: Accepted

## Context

Everything koment does today assumes exactly one repository, discovered by
walking up from the working directory. `store.FindRoot` returns a path; every
other package takes that path for granted. The word "repository" appears in the
code only as `root string`.

A single deployment serving several repositories cannot work that way. It needs
to know which repository an annotation belongs to, keep annotation data
isolated per repository, track sync state per repository, and let a reader move
between them without ambiguity.

The temptation is to make the deployment's store the multi-repository store —
one database, a `repository_id` column, done. That is option 2 of ADR 0016 by
another name, and it takes the reasoning out of the repositories. Whatever
"multi-repository" means, it must not mean that a clone stops containing its own
annotations.

## Decision

The repository stays the unit of storage. The deployment gains an index over
repositories; it does not become the store.

```
Deployment
└── Repository          id, name, clone URL, default branch, config, sync state
    └── Commit          the git context an annotation was created against
        └── File        path at that commit
            └── Annotation
```

Each repository keeps its own `.koment/annotations/` exactly as ADR 0002
specified. Nothing about the on-disk format changes, and a single-repository
checkout behaves identically to today.

The deployment adds, per repository:

| | |
|---|---|
| `id` | stable, assigned once, never derived from the URL |
| `name` | display only |
| `clone_url` | canonical remote |
| `default_branch` | which branch the annotations are read from |
| `config` | per-repository settings |
| `sync` | last indexed commit, last error, independent of other repositories |

The id is assigned rather than derived because remotes move — a repository that
migrates host keeps its annotations, its history and its identity. Deriving the
id from the URL would silently fork a repository's data the day it moved.

**Isolation is structural, not a filter.** A repository's annotations live in
that repository. There is no shared table where a missing `WHERE repository_id`
leaks one project's reasoning into another's view. Cross-repository search is a
fan-out over per-repository stores, which is slower and cannot leak by omission.

`store.Store` already carries a root and resolves paths against it. It becomes
the per-repository handle; a new type owns the set of them. Every UI screen
carries repository context, because the scope is in the data model rather than
in a session variable.

## Consequences

- One deployment can serve many repositories, with annotation data that cannot
  bleed between them by accident.
- **The single-repository case is unchanged.** The CLI, the MCP server and
  `koment ui` against one checkout behave exactly as before. Multi-repository is
  an addition, not a migration, and nobody is forced through it.
- Cross-repository search costs a fan-out. At the scale of one team's
  repositories this is fine; it will need an index long before it needs a
  database, and the index can be a cache because the repositories remain
  authoritative.
- The deployment needs a working copy of every repository it serves, so it needs
  clone credentials, disk, and a sync loop that can fail per repository without
  taking the others down. That is a real operational surface that does not exist
  today.
- Repository ids must be persisted somewhere outside the repositories — a
  deployment-level config. That file is now the one piece of state whose loss
  costs identity rather than data, which makes it the thing to back up.
- Per-repository permissions become possible later without reshaping anything,
  because the scope is already the natural boundary.

## Alternatives rejected

- **One database keyed by `repository_id`.** The conventional answer, far
  simpler to query, and it makes cross-repository search trivial. Rejected
  because it moves annotations out of the repositories, contradicting ADR 0002
  and ADR 0004 and reintroducing the failure ADR 0016 was careful to avoid.
  Isolation would become a predicate that one forgotten clause can defeat.
- **Derive the repository id from the clone URL.** No state to persist and no
  id to assign. Rejected because the URL is not stable: moving host, renaming
  the org, or switching SSH to HTTPS would each fork the repository's identity
  and orphan its sync state.
- **One deployment per repository.** Perfect isolation, no new model at all, and
  it is what the current design already supports. Rejected because it multiplies
  the operational cost by the number of repositories and makes cross-repository
  search impossible, which was a stated requirement.
- **Nest repositories under an organisation or team level now.** Likely needed
  eventually. Left out deliberately: nothing today distinguishes two
  repositories by owner, and inventing a hierarchy before it has a use produces
  the wrong hierarchy. The repository id is stable, so a level can be added
  above it later without touching annotation data.
