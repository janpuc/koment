# 0102 — Build every read surface from one repository snapshot

Date: 2026-08-03
Status: Accepted

## Context

v0.2 has independent translation and traversal paths for CLI, UI, MCP, static
generation, metrics and the database index. That split already loses ambiguity
and author provenance on some surfaces. Static generation resolves the whole
repository once per page, while the database index is used only by manual
index/export commands despite documentation saying that servers query it.

The SQLite freshness stamp misses same-size changes at the same second and does
not observe YAML changes. Its FTS refresh accumulates rows and its schema reset
keeps old table shapes. No live database exists, so compatibility is not a
constraint.

## Decision

Introduce one `RepositorySnapshot` application model containing repository
identity, source commit, annotated source content and fully resolved annotation
views. An annotation view carries the record, resolution, occurrence count,
author claim, verification and historical Git context.

CLI, UI, MCP, static output, search and metrics consume this model. Warning and
presentation policy is defined once.

Build snapshots differently by tier:

- local commands read the current working tree directly;
- static publishing builds one immutable snapshot per invocation;
- served deployments build an immutable snapshot and search index from one
  provider commit, then atomically replace the active in-memory pointer.

Remove the current SQLite index, `koment index` and the claim that `koment
export` can recover Git from a derived cache. Local search uses an in-memory
index built from the snapshot. Served search uses the same disposable shape and
does not introduce a database.

## Consequences

- All readers agree on status, provenance and warnings.
- Static generation becomes linear in annotated files rather than pages times
  files.
- Local use loses a persistent cache but also loses a large SQLite dependency
  and invalidation protocol.
- A failed served refresh leaves the previous complete snapshot active rather
  than exposing a partially built repository.
- Restarting a served process rebuilds snapshots from Git before readiness; no
  cache survives as an alternative recovery source.
- Git remains the only recovery source for annotations.

## Alternatives rejected

- **Repair and wire the current SQLite/Postgres index.** That preserves code but
  carries a local cache and live-filesystem invalidation model into a served
  architecture that needs commit consistency.
- **Store served generations and full-text search in Postgres.** Transactions
  coordinate replicas, but every row is reconstructible from a named Git
  commit. It adds migrations, backup expectations and an operational dependency
  before measured repository size or query latency justifies one.
- **Let each surface read YAML independently.** Simple locally, but duplicated
  policy has already produced materially different answers.
- **Make the database authoritative and export back to Git.** It reverses the
  product's review and durability model and makes a stale read model a recovery
  source.
