# 0104 — Serve assigned repositories as atomic commit snapshots

Date: 2026-08-03
Status: Accepted

## Context

v0.2 can route requests among configured local roots, but clone URL and default
branch are metadata only. The chart mounts or clones one repository once, and
repository identity is still vulnerable to being confused with a checkout
path. Independently updated pod working trees cannot guarantee that one response
resolves and renders a single commit.

A served multi-repository product needs to say which revision every answer
describes and replace a repository coherently when that revision changes.
It also needs to feel like one working product. A repository-selection landing
page makes the normal read path look like a fixture chooser and forces agents
to discover identity before they can use their current workspace.

## Decision

Assign every served repository an immutable id. Configuration also names its
display name, provider repository, default branch and credential reference.
Filesystem paths and cache locations never define identity.

Synchronize repositories outside request handling. Resolve one immutable
provider commit, validate its records, fetch only annotated source blobs and
build the complete snapshot and search index away from readers. Replace the
repository's active in-memory pointer only after that build succeeds. A reader
therefore sees either the old complete commit or the new complete commit, never
a mixture.

Keep source content only for annotated files in the immutable snapshot. Request
handling requires no checkout and performs no provider calls. Repository id
scopes every snapshot, URL, search result, metric and provider operation.

Replicas may refresh at different times, but every response names the exact
commit it serves. A deployment requiring one globally simultaneous revision
uses one active replica; a database is not introduced solely to coordinate a
rebuildable cache.

An unscoped get that matches several repositories refuses and lists the
candidates. An unscoped search may span repositories because every result names
its repository.

Configure one default repository for each published or served repository set.
The human root opens that repository's normal view immediately. A persistent
header switcher changes repository context without becoming a separate landing
page, and direct links include the repository id. Agents derive their default
from the local workspace or authenticated served session; they provide a
repository id only when switching context or using a cross-repository
operation. Repository discovery remains available but is never a startup gate.

## Consequences

- Request handlers read immutable state and agree internally on source and
  resolution.
- Snapshot memory grows with annotated source files rather than entire
  repositories.
- Updates are snapshot-based rather than live working-tree reads.
- Repository synchronization, credentials and failed-generation visibility
  become explicit operational responsibilities.
- A bad new commit leaves the previous valid snapshot available and surfaces
  synchronization failure instead of publishing a partial result.
- Process restart requires rebuilding configured snapshots before readiness.
- Replicas can briefly expose different, explicitly stamped commits.
- The configuration must reject a repository set with no default or more than
  one default.

## Alternatives rejected

- **Mount working trees into every replica.** Simple for one repository, but
  updates can change files while a snapshot is being built and request handling
  inherits checkout and filesystem state.
- **Write complete generations to Postgres.** It gives replicas one activation
  point, but persists source and search data that are completely reconstructible
  from a named commit. The migration, backup and availability burden has no
  measured need before a live deployment exists.
- **Derive identity from root path or clone URL.** Moving a checkout or remote
  renames repository state without an intentional identity change.
- **Fetch source from the forge during each request.** Avoids stored source but
  adds network latency, rate limits and a new partial-failure mode to every read.
- **Deploy one service per repository.** Strong isolation, but loses
  cross-repository discovery and search and multiplies operational overhead.
- **Start on a repository selector.** Makes repository identity explicit, but
  adds a mandatory navigation step and makes maintained content resemble a
  collection of demos.
- **Guess the repository from an unscoped path.** Convenient when paths are
  unique, but becomes nondeterministic as soon as two repositories share one.
