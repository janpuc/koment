# 0022 — Index annotations in a database; git keeps the record

Date: 2026-08-02
Status: Accepted

Amends the storage alternatives rejected in ADR 0002 and ADR 0004. Neither is
reversed: both rejected a database as the **record**, and this ADR adds one as
an **index**. The distinction is the whole decision.

## Context

Every read today is a directory walk. `AnnotatedFiles` walks
`.koment/annotations`, and anything that needs more than one file's annotations
— `check`, `list`, `koment_search`, the UI's file tree, the metrics sweep —
re-reads and re-parses every record. Search is a substring scan over every body.

That is fine at this repository's scale and it does not survive the roadmap:

- **Multi-repository** (ADR 0017) turns cross-repository search into a fan-out
  over N walks, and every screen carrying repository context means every screen
  paying for one.
- **A scalable tree with filtering** needs "files under this directory, with
  status counts, matching this filter" as a query. Built from a walk, that is
  the whole store re-read on every keystroke.
- **Historical views** (ADR 0014) will want "annotations created before commit
  X", which a per-file walk cannot answer at all without reading everything.

So koment needs a query engine. The question is what becomes authoritative.

Committing a SQLite file to git was considered and is the reason this ADR is
careful. A database file in git is binary: unreviewable in a pull request,
unmergeable when two people annotate the same repository (there is no merge
driver for SQLite, so one side's work is simply lost), and it grows history on
every write because a page change rewrites the blob. That is precisely the
objection ADR 0002 recorded against a single store file, and it is still
correct.

## Decision

**Git keeps the record. The database does the work.**

- `.koment/annotations/**.yaml` remains the source of truth, unchanged. It is
  what a pull request reviews, what merges, and what a clone contains.
- A **derived index** — SQLite by default, Postgres optionally — holds the same
  annotations in queryable form. Every read path goes through it.
- The index is rebuilt from YAML, deterministically. It is never edited
  directly and never the only copy of anything.

```
.koment/annotations/**.yaml   record      reviewed, merged, cloned
        ↓ rebuild (deterministic)
        index                 derived     queried, searched, filtered
```

**Location.** SQLite lives in the user's cache directory, keyed by repository,
and is gitignored. It is a build artifact. A missing, stale or corrupt index is
never a data-loss event — it is a rebuild.

**Postgres makes the service stateless.** With `KOMENT_DATABASE_URL` set the
index lives in the database, no replica holds local state, and any of them can
serve any request. That is what a multi-repository deployment needs and what a
local SQLite file cannot give.

**Resolution stays live.** This is the subtle part. Annotation *content* comes
from the index; resolution *status* depends on source files that change between
requests, and ADR 0013 promises the UI shows what is on disk. So the index
caches each file's resolution against its `(mtime, size)`, and any file whose
stamp has changed is re-resolved before it is served. A stale status is never
shown; an unchanged file is never re-read.

**Dependencies.** Both verified 2026-08-02:

- `modernc.org/sqlite` — pure Go, so it builds under `CGO_ENABLED=0` onto the
  distroless static base the Dockerfile already uses. A cgo driver
  (`mattn/go-sqlite3`) was disqualified before evaluation for that reason alone.
  FTS5 is present, which is the search backend.
- `github.com/jackc/pgx/v5` — the Postgres driver, used through `database/sql`
  so the two are swappable behind one interface.

Both are confined to `internal/index`.

## Consequences

- Search, filtering and the file tree become queries. The UI work this unblocks
  is the reason to do it now rather than after.
- **Nothing about reviewability changes.** Annotations still diff line by line
  in a pull request, still merge, and a clone still contains its own reasoning
  with no database anywhere.
- Two representations, and the risk that they disagree. Mitigated structurally:
  the index is derived and rebuilt, never written to directly, so "disagree"
  means "rebuild" rather than "reconcile". A `koment index --rebuild` forces it.
- Two more direct dependencies, taking the total to five. That is a real cost
  against AGENTS.md §10 and the largest single jump so far.
- Rebuild cost is O(annotations) at startup. Milliseconds here; for a very large
  repository it becomes a startup delay, and the mtime cache means it is paid
  once rather than per request.
- The CLI (`add`, `check`, `reanchor`) keeps reading YAML directly and needs no
  index. A laptop with no database still works exactly as it does today, which
  is what keeps ADR 0004's "reading a checkout must never depend on a network
  call" true.

## Alternatives rejected

- **SQLite committed to the repository as the source of truth.** What was
  initially asked for, and it would remove the two-representation problem
  entirely. Rejected on three independent grounds, any one of which is
  disqualifying: a binary file cannot be reviewed in the pull request that
  changes it, two people annotating the same repository produce a conflict git
  cannot merge and one loses their work, and every write grows history by a
  whole blob. ADR 0002 chose per-file YAML for exactly these reasons.
- **Database as the sole record, not in the repository at all.** The cleanest
  data model, genuinely stateless, no sync. Rejected because a clone would stop
  containing its own reasoning — the founding premise of the project, and the
  thing ADR 0004 protected.
- **Keep walking the directory; optimise later.** No dependency, no second
  representation, and correct today. Rejected because the roadmap's next step is
  a tree with live filtering over multiple repositories, and building that on a
  full re-read per interaction is knowingly writing something that has to be
  torn out.
- **An in-memory index, rebuilt at startup, no database.** No dependency at all,
  and it would serve the single-repository UI well. Rejected because it cannot
  make a multi-replica deployment stateless, and because full-text search would
  then be hand-rolled — which is the SQLite FTS5 that comes free.
- **Caching resolution status in the index without an invalidation stamp.**
  Faster and much simpler. Rejected outright: it would serve a status that no
  longer matches the working tree, which is the exact failure this whole project
  exists to make impossible.
