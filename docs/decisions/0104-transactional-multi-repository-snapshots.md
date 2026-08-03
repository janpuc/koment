# 0104 — Serve assigned repositories as transactional commit snapshots

Date: 2026-08-03
Status: Accepted

## Context

v0.2 can route requests among configured local roots, but clone URL and default
branch are metadata only. The chart mounts or clones one repository once. Its
database identity is derived from the checkout path even though the registry
assigns an id. A Postgres row plus independently cloned pod working trees cannot
guarantee that every replica resolves and renders the same commit.

A served multi-repository product needs to say which revision every answer
describes and replace a repository coherently when that revision changes.

## Decision

Assign every served repository an immutable id. Configuration also names its
display name, canonical clone URL, default branch and credential reference.
Filesystem paths never define identity.

Synchronize repositories outside request handling. For one fetched commit, an
ingester validates records, reads the annotated source files, resolves every
anchor and writes a complete Postgres generation in one transaction. A reader
selects one active generation per repository and therefore sees either the old
commit or the new commit, never a mixture.

Store source content only for annotated files with the generation. Read replicas
serve from Postgres and require no local checkout. Repository id scopes every
database key, URL, search result, metric and outbox record.

An unscoped get that matches several repositories refuses and lists the
candidates. An unscoped search may span repositories because every result names
its repository.

## Consequences

- Read replicas are genuinely stateless and agree on source and resolution.
- Ingestion storage grows with annotated source files rather than entire
  repositories.
- Updates are snapshot-based rather than live working-tree reads.
- Repository synchronization, credentials and failed-generation visibility
  become explicit operational responsibilities.
- A bad new commit leaves the previous valid generation available and surfaces
  ingestion failure instead of publishing a partial result.

## Alternatives rejected

- **Mount working trees into every replica.** Simple for one repository, but
  replicas can update at different times and the chart cannot make Postgres
  identify their local commit.
- **Derive identity from root path or clone URL.** Moving a checkout or remote
  renames repository state without an intentional identity change.
- **Fetch source from the forge during each request.** Avoids stored source but
  adds network latency, rate limits and a new partial-failure mode to every read.
- **Deploy one service per repository.** Strong isolation, but loses
  cross-repository discovery and search and multiplies operational overhead.
