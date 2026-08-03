# koment — target design

Status: **approved target; implementation in progress.**

The v0.2 implementation remains usable while this design is built, but it is
not the specification. The active architectural decisions start at ADR 0100 in
`docs/decisions/`. Earlier decisions describe the pre-deployment prototype and
remain available in Git history.

## Thesis

Code should explain what it does through names, types and structure. Inline
comments often narrate code, drift silently and make the code harder to edit.
Removing them creates a different need: humans and agents must still be able to
record why a choice was made, what failed before and which invariant must
survive a change.

koment stores that local reasoning outside the source file, keeps it in Git and
binds it to code with a deterministic anchor. The same record must be easy for
a human or an agent to add, find and judge. A stale or ambiguous record is
shown as such everywhere; no surface may silently turn uncertainty into fact.

## Principles

1. **Git is the record.** The committed YAML is authoritative. Databases,
   static sites and in-memory search structures are disposable read models.
2. **One fact has one representation.** An annotation is one record with one
   stable id, regardless of which surface created or reads it.
3. **Resolution is deterministic.** Exact text and captured context decide an
   anchor. Lines describe movement but never choose identity.
4. **Uncertainty fails loudly.** Drift, orphaning and ambiguity are failures.
   Every surface carries the same status and warning.
5. **Humans and agents are peers.** Both can read and write through first-class
   interfaces backed by the same application service.
6. **Repository identity is assigned.** A checkout moving on disk does not
   create a new repository.
7. **Remote access is authenticated.** Read-only source and rationale are still
   confidential. Loopback is the only unauthenticated network boundary.
8. **A published page is a snapshot.** It names its commit and never pretends to
   be live or writable.
9. **The implementation dogfoods the thesis.** Local rationale belongs in
   koment, structural rationale in ADRs and inline comments are exceptional.
10. **Operations follow konflate's standard.** Tooling, CI, containers, Helm
    and releases use `home-operations/konflate` as their baseline unless an ADR
    records a deliberate difference.

## Implementation status

This table is the honest boundary between the current v0.2 code and the target.

| Capability | Current state | Target state |
|---|---|---|
| Git-backed annotations | implemented as record lists mirrored per source file | one record per annotation |
| Deterministic excerpts | implemented | context disambiguation and explicit `ambiguous` failure |
| CLI read and write | implemented | rebuilt on the common application service |
| Local human UI | read-only | read and explicit loopback write mode |
| Local agent MCP | read-only | read and explicit stdio write mode |
| Static publishing | partial | atomic commit snapshot with body search and JSON |
| Multi-repository routing | partial | assigned identity plus synchronized commit snapshots |
| HTTP serving | separate unauthenticated UI or MCP | one authenticated human-and-agent service |
| Database index | prototype, not used by servers | new Postgres read model for served snapshots only |
| Remote authoring | design only | authenticated exact outbox materialized through Git |
| Helm and release | baseline exists | konflate-aligned tests, hardening and signatures |

## Annotation record

Each annotation is stored at `.koment/annotations/<id>.yaml`. The filename and
the record id must agree. Adding two annotations creates two files, so
concurrent humans and agents do not perform a shared read-modify-write.

```yaml
version: 2
id: 01JQ8ZK3M4N5P6R7S8T9V0W1X2
file: internal/session/token.go
kind: invariant
body: |-
  Rotation must keep the previous key until every token minted before the
  rotation window has expired.
created: "2026-08-03"
anchor:
  scope: excerpt
  excerpt: "if token.Expired(now) {"
  before: |-
    func validate(token Token, now time.Time) error {
  after: |-
        return ErrExpired
  last_seen_line: 42
git:
  commit: 9f3c1a4d8e2b7c5a6f0d3e1b8c7a5f2d4e6b9c1a
  path: internal/session/token.go
  line: 42
  end_line: 42
author:
  name: Jan Pucilowski
  kind: human
  source: git-config
```

Required fields are `version`, `id`, `file`, `kind`, `body`, `created`,
`anchor` and `author`. Git context is recorded when available and never affects
resolution. Author kind is `human`, `agent` or `unknown`; new writes require the
first two. An imported v1 annotation with no attributable author records an
explicit `unknown` legacy identity; migration never invents a person.

Kinds remain deliberately constrained:

- `why` — why this approach won;
- `gotcha` — surprising behaviour a changer must account for;
- `invariant` — a property that must remain true;
- `anti-pattern` — an attractive approach already found to be wrong.

The YAML decoder rejects unknown fields. Record serialization is deterministic
so a semantic no-op does not create a Git diff. ADR 0100 records the storage
decision.

## Anchors and resolution

An anchor has `file` or `excerpt` scope. File scope applies to the whole file.
Excerpt scope stores exact text plus up to three complete source lines
immediately before and after it, captured automatically at creation or reanchor
time. Fewer lines are stored at a file boundary. Users do not hand-maintain
hashes or context.

Resolution follows one order:

1. If the file does not exist, return `orphaned`.
2. File scope returns `ok`.
3. Find every exact occurrence of the excerpt.
4. No occurrence returns `drifted`.
5. One occurrence resolves there.
6. Several occurrences are filtered by the captured before and after context.
7. Exactly one contextual candidate resolves there; otherwise return
   `ambiguous`.
8. A resolved line equal to `last_seen_line` is `ok`; another line is `moved`.

| Status | Meaning | `koment check` |
|---|---|---|
| `ok` | the anchor resolves where it was last confirmed | pass |
| `moved` | the anchor resolves uniquely at another line | pass |
| `ambiguous` | more than one candidate remains | fail |
| `drifted` | the file exists but the excerpt does not | fail |
| `orphaned` | the file does not exist | fail |

`last_seen_line` is descriptive metadata. It never selects a candidate.
Reanchor keeps the id, author and creation date, replaces the anchor and records
the newly confirmed line. ADR 0101 records the resolution decision.

All repository reads and writes use a filesystem root that prevents a relative
path or symlink from escaping the repository. Lexical path cleaning is not a
security boundary.

## Shared application model

Storage and presentation are separated by one application model:

```text
RepositorySnapshot
├── repository identity
├── source commit and generation time
└── annotated files
    ├── source content
    └── annotation views
        ├── record
        ├── current resolution and occurrence count
        ├── author claim and verification
        └── historical Git context
```

CLI, UI, MCP, static generation, search and metrics consume this model. They do
not independently translate records or invent warnings. A status, author or
provenance field visible in one read surface is visible in all applicable read
surfaces.

Local commands build a snapshot directly from the current working tree.
`koment site` builds one immutable snapshot for the whole render. The served
tier reads transactional snapshots from Postgres. The current bidirectional
SQLite index and its recovery role are removed; Git alone recovers records.
ADR 0102 records this boundary.

## Product tiers

The tiers share records and presentation, not false capability parity.

| Tier | Source | Human read | Agent read | Write | Repository scope |
|---|---|---|---|---|---|
| local | current working tree | CLI and UI | stdio MCP | direct Git records | one or configured local set |
| published | one commit snapshot | static UI | static JSON/search data | none | one repository per site |
| served | transactional snapshots | authenticated UI | authenticated MCP | authenticated outbox | many assigned repositories |

### Local

The CLI remains the universal interface:

```text
koment add <file> [--excerpt <text>] --kind <kind> --body <text>
koment show <file>
koment list
koment search <query>
koment check [path...]
koment reanchor <id> [--excerpt <text>] [--file <path>]
koment ui [--write]
koment mcp [--write]
```

`koment ui --write` is loopback-only, uses an unguessable session capability
and rejects cross-origin writes. `koment mcp --write` is allowed over stdio.
Write tools are not registered on an unauthenticated HTTP transport.

The MCP surface is symmetrical with the application service:

- `koment_repositories`
- `koment_get`
- `koment_search`
- `koment_add` when writes are enabled
- `koment_reanchor` when writes are enabled

Every mutation records human or agent authorship honestly and returns the full
record plus its repository-relative YAML path.

### Published

`koment site` writes a complete site to a staging directory and publishes it by
replacement. Rebuilding cannot leave pages from a previous snapshot.

The output contains the human UI, annotation-body search data and an
`annotations.json` machine-readable snapshot. Every page names the repository
and commit. Static output is read-only by nature; it may link to a configured
writer but never presents an inert write control.

### Served

`koment serve` exposes one coherent service:

```text
/          human UI
/mcp      agent MCP
/livez    process health
/readyz   dependency and snapshot readiness
```

Prometheus metrics remain on a separate listener. The process derives its root
context from termination signals, bounds shutdown and treats a configured
listener that cannot start as fatal. MCP requests and sessions have explicit
limits.

Every non-loopback request is authenticated. Human identity comes from a
trusted OIDC boundary. Agents use scoped bearer credentials. The application
accepts forwarded identity only from configured trusted proxies and records the
verification mechanism with the author claim. ADR 0103 defines tier and
surface capability; ADR 0105 defines remote writes.

## Multi-repository serving

A served repository is configured with an immutable id, display name,
canonical clone URL and default branch. Local filesystem paths are deployment
details and never identity.

```yaml
repositories:
  - id: payments
    name: Payments API
    clone_url: https://github.com/example/payments
    default_branch: main
```

An ingester synchronizes each repository outside the request path:

1. Fetch and check out one commit.
2. Read and validate every annotation record.
3. Read only the source files that have annotations.
4. Resolve every anchor and build the repository snapshot.
5. Replace that repository's database generation in one transaction.

Readers see the previous complete generation or the next complete generation,
never rows assembled from different commits. Source content for annotated files
is stored with the generation, so read replicas need no local checkout and are
actually stateless.

Search, URLs, metrics, outbox rows and database keys all carry the assigned
repository id. Cross-repository search names the repository of every result.
When a file path exists in several repositories, an unscoped get refuses and
names the candidates. ADR 0104 records the multi-repository decision.

## Remote authoring and Git settlement

Remote creation never edits a read replica's checkout and never pushes directly
to a default branch. An authenticated request creates an exact v2 record in an
outbox with its stable id, repository id, base commit and author identity.

```text
created ──▶ pending ──▶ pull request ──▶ settled
                │              │
                └── conflict ◀─┘
```

A materializer, implemented behind a provider interface with GitHub first,
creates or updates a branch and pull request containing the YAML record. Once a
synced repository snapshot contains that id with the same record content, the
outbox entry is settled and removed. A conflicting committed record stops
materialization and remains visible as a conflict; Git wins because it is the
record.

The outbox stores exact records, not summaries or embeddings. It does not merge,
deduplicate, demote or expire rationale. ADR 0105 records this lifecycle.

## Search and read models

Search has one contract and tier-specific implementations:

- local processes build an in-memory index from one snapshot;
- published sites include a generated static search dataset;
- served deployments use Postgres full-text search scoped by repository
  generation.

Search covers bodies, file paths, kinds, authors and ids. Results return the
same annotation view as `get`, including resolution and provenance. A read
model can always be discarded and rebuilt from Git plus the source commit; it
is never a recovery source for Git.

## Security boundaries

- Repository file access cannot escape its opened root through paths or
  symlinks.
- Loopback local services may be unauthenticated; non-loopback services may not.
- Browser writes require same-origin and CSRF protection.
- Agent credentials are scoped to repositories and read or write capability.
- Sensitive configuration comes from secret references, never chart values
  rendered into pod specifications.
- Request bodies, sessions, concurrent work and graceful shutdown are bounded.
- Static publishing replaces output atomically and cannot retain removed files.
- Remote Git writes go through reviewable pull requests.

## Operations

konflate is the operational baseline, inspected at the commit recorded in ADR
0106. koment adopts its pinned toolchain, local task runner, workflow linting,
vulnerability scanning, Helm tests, container hardening, digest pinning and
release signing. Differences must be deliberate:

- koment retains race testing;
- koment has no Node frontend toolchain;
- rationale that konflate places in inline comments belongs in koment
  annotations or ADRs here.

The Helm chart deploys `koment serve`, not mutually exclusive human and agent
modes. It provides a values schema, generated documentation, a non-token-bearing
service account, probes, optional NetworkPolicy and disruption controls. CI
installs the chart into Kind and runs `helm test` against the built image.

Images and charts are digest-addressable and signed. Binary checksums are
authenticated rather than downloaded unsigned beside the binary they verify.

## Comment-free dogfooding

Before adding a comment, contributors rename, extract, introduce a named type or
constant, and restructure in that order. Remaining local rationale becomes a
koment annotation; structural rationale becomes an ADR.

Allowed inline comments are toolchain directives, links required to explain an
external constraint, deprecation markers and genuine public API documentation.
An AST-aware CI check enforces the Go rule. Other source-like files are audited
without blindly deleting schema directives or generated documentation markers.
ADR 0107 records why koment enforces its own thesis.

## Implementation sequence

### 1. Operational floor

Adopt the pinned konflate-style toolchain and missing CI security checks before
large code changes. Retain the existing race suite.

### 2. Record and anchor v2

Implement one-record-per-annotation storage, migrate this repository's records,
add ambiguity and context resolution, enforce rooted filesystem access and
remove the current index/export subsystem.

### 3. Shared reads and local writes

Introduce the snapshot and application services, move every reader to them,
surface provenance consistently, add local UI and MCP writes, and rebuild static
publishing and search.

### 4. Served multi-repository system

Add the unified server, authentication, Postgres generations, repository
ingestion, exact outbox and GitHub materializer.

### 5. Deployment and release

Replace the prototype chart modes, add schema and E2E coverage, then sign and
digest-pin all release artifacts.

Each stage leaves tests and documentation describing only behaviour that
exists. A capability moves from `planned` to `implemented` in this document in
the same change that verifies it.

## Definition of done

vNext is complete when:

1. Concurrent local agents can add and reanchor records without losing work.
2. All five resolution statuses have real before/after fixtures and identical
   presentation across CLI, UI, MCP and static output.
3. No repository-controlled path or symlink can expose a file outside its root.
4. Humans and agents can read and write locally through first-class surfaces.
5. Static output is atomic, searchable, commit-stamped and machine-readable.
6. One authenticated service presents UI and MCP for several assigned
   repositories from transactional commit snapshots.
7. Remote writes retain exact content and author identity through a reviewed Git
   pull request.
8. The Helm E2E test installs the built image and passes health and functional
   probes in Kind.
9. Vulnerability, workflow, annotation, comment-policy and race checks pass in
   CI.
10. koment's own source has moved non-exempt rationale out of inline comments
    and its annotations resolve.

## Non-goals

- LLM-generated annotations or semantic reanchoring.
- Consolidating, summarizing or expiring rationale as a memory system.
- Tree-sitter or language-specific symbol anchors before deterministic v2 is in
  real use.
- A writable static site.
- Direct pushes from the served tier to a default branch.
- A full forge abstraction before the GitHub implementation proves the
  interface.
- An IDE plugin before the local and served application services are stable.

## Prior art

- [konflate](https://github.com/home-operations/konflate) — operational and
  deployment baseline.
- [Codetations / Magic Markup](https://github.com/elmisback/codetations) —
  document-external annotation research.
- [Robustly Anchoring Annotations Using Keywords](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/tr-2001-107.pdf)
  — deterministic anchoring prior art.
- [Architecture Decision Records](https://adr.github.io/) — project-wide
  decision history; koment records rationale local to code.
