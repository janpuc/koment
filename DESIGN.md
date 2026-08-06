# koment — design

Status: **approved; product and reference integrations implemented, external catalog acceptance pending.**

This document is the specification. The active architectural decisions start at
ADR 0100 in `docs/decisions/`. Earlier decisions describe the pre-deployment
prototype and remain available in Git history.

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

1. **Git is the record.** The committed YAML is authoritative. Static sites,
   filesystem caches and in-memory search structures are disposable read models.
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
11. **Comment intent meets users where they are.** Writing ordinary explanatory
    commentary should create a koment annotation. Keeping it inline requires an
    attributable, reviewable acknowledgement.
12. **Agent obedience is not the boundary.** Repository instructions, MCP
    guidance and client hooks make the correct workflow immediate, while a
    required policy check decides what may land regardless of which agent made
    the edit.
13. **End users install artifacts, not a Go program.** Contributor workflows
    may invoke the Go toolchain, but installation, CI integration, agents and
    editors consume published, authenticated koment artifacts.
14. **Rebuildable served state stays ephemeral.** A named Git commit contains
    everything required to rebuild a repository snapshot. koment does not add a
    database merely to persist data that Git already owns.

## Implementation status

This table is the honest boundary between implemented and planned behavior.

| Capability | Current state | Target state |
|---|---|---|
| Git-backed annotations | one record per annotation implemented | implemented |
| Deterministic excerpts | context disambiguation and explicit `ambiguous` failure implemented | implemented |
| CLI read and write | common application service, search and comment policy implemented | implemented |
| Local human UI | read and capability-gated loopback write mode implemented | implemented |
| Local agent MCP | read and explicit stdio write mode implemented | implemented |
| Static publishing | atomic commit snapshot, body search and JSON implemented | implemented |
| Multi-repository routing | assigned identity, commit snapshots and contextual switching implemented | implemented |
| HTTP serving | authenticated UI and MCP share one service and snapshot catalog | implemented |
| Database index | local prototype removed | none; served snapshots and search remain rebuildable in memory |
| Remote authoring | authenticated creation materializes exact records through reviewed Git pull requests | implemented for creation; source-mutating operations remain local |
| Agent policy | strict policy, generated client adapters, hooks and CI gate implemented | implemented |
| Operational toolchain | mise, Lefthook, generated chart documentation, security gates and a self-hosted Renovate workflow implemented; Renovate stays inert until its app is installed | implemented |
| Helm and release | hardened chart, Kind E2E, signed canonical artifacts and SBOM/provenance implemented | implemented |
| End-user distribution | releases, setup Action, mise, Claude marketplace, MCP metadata, VS Code/Open VSX package, and generated package-manager manifests | external catalog and publisher account acceptance pending |
| Editor presentation | inline gloss plus a panel carrying full bodies; diagnostics report only what fails `koment check` | implemented |
| Editor distribution | seven signed packages per release — six carrying the platform's canonical binary, one universal — ordered after the binaries job, with opt-in marketplace publication and LSP configuration for every other editor | implemented; marketplace publication awaits publisher accounts |
| Windows | archives, checksums, signatures and package manifests ship; an advisory job installs and runs the published archive | supported and non-gating by decision (ADR 0111) |
| Maintained workspace | builds, tests, publishes and carries current annotations | implemented |

## Annotation record

Each annotation is stored at `.koment/annotations/<id>.yaml`. The filename and
the record id must agree. Adding two annotations creates two files, so
concurrent humans and agents do not perform a shared read-modify-write.

The repository publishes one strict JSON Schema for the annotation record.
Every record written by koment starts with a `yaml-language-server` schema
directive that points to the raw schema on the default branch, and this
repository carries a VS Code YAML
association for the annotation glob. Editors can validate fields, enumerations
and required values without a koment plugin. The directive is tooling metadata,
not rationale, and is allowed by the comment policy. The schema can move to a
pinned published URL once the record has real external users.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/koment-dev/koment/main/schema/v1alpha/annotation.schema.json
apiVersion: koment.dev/v1alpha
kind: Annotation
metadata:
  id: 01JQ8ZK3M4N5P6R7S8T9V0W1X2
  created: "2026-08-03T09:15:00Z"
spec:
  target:
    file: internal/session/token.go
  type: invariant
  title: Rotation keeps the previous key until every token expires
  body: |-
    Rotation must keep the previous key until every token minted before the
    rotation window has expired.
  anchor:
    scope: excerpt
    excerpt: "if token.Expired(now) {"
    before: |-
      func validate(token Token, now time.Time) error {
    after: |-
          return ErrExpired
  git:
    commit: 9f3c1a4d8e2b7c5a6f0d3e1b8c7a5f2d4e6b9c1a
    path: internal/session/token.go
    line: 42
    end_line: 42
  author:
    name: Jan Pucilowski
    kind: human
    source: git-config
status:
  lastSeenLine: 42
  resolution: ok
  resolvedAt: "2026-08-03T09:15:00Z"
  resolvedCommit: 9f3c1a4d8e2b7c5a6f0d3e1b8c7a5f2d4e6b9c1a
```

The record is a Kubernetes-shaped resource. `apiVersion` names the API group
and its generation, `kind` names the resource, `metadata` identifies it, `spec`
is what the author decided and `status` is what the last write observed. ADR
0119 records the shape and the divergence from Kubernetes on
`metadata.id`: a ULID is not a DNS-1123 name, so koment does not call it
`metadata.name`.

Required fields are `apiVersion`, `kind`, `metadata.id`, `metadata.created`,
`spec.target.file`, `spec.type`, `spec.body`, `spec.anchor` and `spec.author`.
Git context is recorded when available and never affects resolution. Author
kind is `human`, `agent` or `unknown`; new writes require the first two. An
imported prototype annotation with no attributable author records an explicit
`unknown` legacy identity; migration never invents a person.

`status` is written by the commands that already write a record — `add`,
`reanchor` and their MCP equivalents — and never by a read. Nothing consults it
to decide where an annotation applies: a reader resolves the anchor against the
file in front of it. `status.resolvedCommit` exists so that a reader can see how
old the recorded observation is, and `status.resolvedAt` answers "since when has
this been true" rather than "when did a command last run".

A record written before ADR 0119 carried a flat `version: 1` shape. Any binary
from 1.0.0 onward rewrites such a record in the current shape the first time it
reads it. The rewrite is atomic and idempotent; a repository mounted read-only
still reads correctly, because the upgrade happens in memory and only the
rewrite is skipped.

Types remain deliberately constrained:

- `why` — why this approach won;
- `gotcha` — surprising behaviour a changer must account for;
- `invariant` — a property that must remain true;
- `anti-pattern` — an attractive approach already found to be wrong.

The YAML decoder rejects unknown fields. Record serialization is deterministic
so a semantic no-op does not create a Git diff. ADR 0100 records the storage
decision.

An annotation that authorizes an otherwise forbidden inline comment adds this
machine-readable policy acknowledgement:

```yaml
spec:
  type: why
  body: |-
    The generator requires this marker at the declaration it controls.
  anchor:
    scope: excerpt
    excerpt: "// generator:keep"
  policy:
    exception: inline-comment
    acknowledged: true
status:
  lastSeenLine: 18
```

The anchor must resolve to the exact comment, not merely nearby code. Its body
explains why renaming, extraction, a named type or constant, restructuring and
an ordinary koment annotation were insufficient. Author and creation fields
make the acknowledgement attributable. Changing or removing the comment makes
the acknowledgement drift or orphan instead of silently broadening it.

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
8. A unique resolution is `ok`, wherever it was found. `status.lastSeenLine`
   is not consulted.

| Status | Meaning | `koment check` |
|---|---|---|
| `ok` | the anchor resolves where it was last confirmed | pass |
| `ambiguous` | more than one candidate remains | fail |
| `drifted` | the file exists but the excerpt does not | fail |
| `orphaned` | the file does not exist | fail |

`status.lastSeenLine` is descriptive metadata. It never selects a candidate.
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
tier builds the same model from a named provider commit and atomically replaces
the active in-memory snapshot only after the complete build succeeds. The
current bidirectional SQLite index and its recovery role are removed; Git alone
recovers records. ADR 0102 records this boundary.

## Product tiers

The tiers share records and presentation, not false capability parity.

| Tier | Source | Human read | Agent read | Write | Repository scope |
|---|---|---|---|---|---|
| local | current working tree | CLI and UI | stdio MCP | direct Git records | one or configured local set |
| published | commit snapshots | static UI | static JSON/search data | none | one repository or a configured set |
| served | atomically replaced commit snapshots | authenticated UI | authenticated MCP | reviewed Git pull requests | many assigned repositories |

### Local

The CLI remains the universal interface:

```text
koment add <file> [--excerpt <text>] --kind <kind> --body <text>
koment show <file>
koment list
koment search <query>
koment check [path...]
koment reanchor <id> [--excerpt <text>] [--file <path>]
koment comments check [path...]
koment comments convert <file> --excerpt <comment> [--kind <kind>]
koment comments acknowledge <file> --excerpt <comment> --body <text> --acknowledge-inline-comment
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
- `koment_convert_comment` when writes are enabled
- `koment_acknowledge_comment` when writes are enabled and its explicit
  acknowledgement input is true

Every mutation records human or agent authorship honestly and returns the full
record plus its repository-relative YAML path.

### Published

`koment site` writes a complete site to a staging directory and publishes it by
replacement. Rebuilding cannot leave pages from a previous snapshot.

The output contains the human UI, annotation-body search data and an
`annotations.json` machine-readable snapshot for each repository. Every page
names the active repository and its commit. A publication can contain one
repository or a configured set of independently stamped repository snapshots.
Its root opens the configured default repository immediately; it never blocks
on a repository-selection landing page. A persistent repository switcher in
the normal application header changes context after the reader is already in
the product.

Static output is read-only by nature; it may link to a configured writer but
never presents an inert write control.

### Human navigation and search

The ordinary repository view owns navigation; there is no repository-selector
landing page. A compact, visually distinct repository switcher sits in the top
right of the application header and preserves the current page context when a
matching route exists. The left rail ends with a persistent source link to
`https://github.com/koment-dev/koment`.

Search opens as a centered modal over the current repository instead of taking
permanent space from source and rationale. A visible search control shows the
platform's primary shortcut: `⌘K` on Apple platforms and `Ctrl+K` elsewhere.
The matching primary modifier opens or focuses search, `/` opens it when focus
is not already in an editable control, arrow keys move through results, Enter
opens the active result and Escape closes the modal. Focus returns to the
control that opened it. The dialog has an accessible name, traps focus while
open and remains fully usable without a keyboard shortcut.

Local UI, static publication and served UI render this same application shell.
Search data and repository routes differ by tier, but navigation placement and
keyboard behavior do not.

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

Served mutation creates a new annotation and materializes it as a dedicated
review pull request. Reanchor, comment conversion and inline acknowledgement
also change an existing record or source file; they remain local CLI, MCP and
editor operations so koment cannot overwrite a remote agent's separate
worktree through an unrelated pull request.

## Multi-repository serving

A served repository is configured with an immutable id, display name, provider
repository and default branch. Local filesystem paths and cache locations are
deployment details and never identity.

```yaml
repositories:
  - id: payments
    name: Payments API
    provider: github
    remote: example/payments
    default_branch: main
    default: true
```

The first served provider is GitHub. Its implementation uses the provider's Git
data APIs rather than requiring a Git executable or a writable checkout in the
application container. A small provider contract may separate synchronization
from presentation, but koment does not claim forge portability before another
provider exists.

A synchronizer refreshes each repository outside the request path:

1. Resolve the configured branch to one immutable commit.
2. Enumerate, read and validate every annotation record at that commit.
3. Read only the source blobs that have annotations.
4. Resolve every anchor and build the complete repository snapshot and search
   index away from readers.
5. Atomically replace that repository's active snapshot pointer.

Readers see the previous complete snapshot or the next complete snapshot, never
data assembled from different commits. A failed refresh leaves the previous
valid snapshot available and makes the failure visible through readiness,
metrics and repository status. Source content for annotated files lives in the
immutable snapshot, so request handling needs no checkout and performs no
provider calls.

Replicas synchronize independently and can briefly serve different commits.
Every response and direct link names its commit, so this is visible staleness
rather than mixed or falsely current data. A deployment that requires one
globally simultaneous revision runs a single active synchronizer and replica;
koment does not introduce a database solely to coordinate a cache.

Search, URLs, metrics, provider operations and snapshot keys all carry the
assigned repository id. Cross-repository search names the repository of every result.
When a file path exists in several repositories, an unscoped get refuses and
names the candidates.

One repository in a configured set is the default. The human UI opens it as an
ordinary repository view and keeps the active repository in a persistent
header switcher; repository discovery is never a separate first-use page.
Direct links retain repository context. Local agents derive the active
repository from their workspace, served agents derive it from their scoped
session, and only cross-repository operations require an explicit repository
id. `koment_repositories` remains available for discovery but is not a startup
gate. ADR 0104 records the multi-repository decision.

## Remote authoring and Git settlement

Remote creation never edits a read replica's checkout and never pushes directly
to a default branch. An authenticated request creates an exact annotation
record with its stable id, repository id, base commit and author identity, then
asks the provider materializer to put that record in a reviewable pull request.

```text
request ──▶ branch commit ──▶ pull request ──▶ merged snapshot
   │               │                 │
   └──────────── conflict ◀───────────┘
```

A materializer, implemented behind a provider interface with GitHub first,
uses a deterministic branch derived from the annotation id and creates or
updates a commit and pull request containing the YAML record. The request does
not report success until the pull request exists. If a provider call fails part
way through, an idempotent retry inspects that branch and resumes instead of
creating a second record or pull request.

The provider's branch, commit and pull request are the durable pending state.
Once a synchronized default-branch snapshot contains that id with the same
record content, the write is settled. A different committed record with the
same id is a visible conflict; Git wins because it is the record. koment does
not keep a second outbox, merge, deduplicate, demote or expire rationale. ADR
0105 records this lifecycle.

## Search and read models

Search has one contract and tier-specific implementations:

- local processes build an in-memory index from one snapshot;
- published sites include a generated static search dataset;
- served deployments query an immutable in-memory index scoped by repository
  snapshot.

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
- Provider hosts are configured explicitly; repository data cannot redirect
  authenticated requests to arbitrary network destinations.
- Provider responses, repository trees and source blobs are bounded before
  allocation or parsing.
- Request bodies, sessions, concurrent work and graceful shutdown are bounded.
- Static publishing replaces output atomically and cannot retain removed files.
- Remote Git writes go through reviewable pull requests.

## Operations

konflate is the operational baseline, inspected at the commit recorded in ADR
0106. koment adopts its pinned toolchain, local task runner, workflow linting,
vulnerability scanning, Helm tests, container hardening, digest pinning and
release signing. Differences must be deliberate:

- koment retains race testing;
- the browser UI has no Node runtime or build toolchain; the VS Code package
  uses a checksum-locked Node toolchain only for tests and marketplace packaging;
- rationale that konflate places in inline comments belongs in koment
  annotations or ADRs here.

The Helm chart deploys `koment serve`, not mutually exclusive human and agent
modes. It provides a values schema, generated documentation, a non-token-bearing
service account, probes, optional NetworkPolicy and disruption controls. It
does not require a database. CI installs the chart into Kind and runs `helm
test` against the built image.

Images and charts are digest-addressable and signed. Binary checksums are
authenticated rather than downloaded unsigned beside the binary they verify.

## Installation and distribution

GitHub Releases are the canonical source for platform archives, checksums,
signatures, SBOMs and provenance. GHCR is canonical for the container and Helm
OCI artifacts. Every package manager, marketplace and registry entry references
or repackages those exact release outputs; it does not compile a second binary.

End-user documentation never instructs a person, CI runner, agent client or
editor to use `go install`, `go run` or a Go build container. Those commands are
permitted only in contributor documentation and repository-owned development
tasks. A channel that cannot consume or authenticate a released artifact does
not ship until it can.

Distribution is promoted in layers:

1. GitHub Releases, the setup Action, GHCR images and Helm OCI artifacts ship
   directly from the release workflow.
2. A maintained Homebrew tap, mise's GitHub backend, WinGet and Scoop install
   the platform archives. An Aqua registry entry supplies the short `mise use
   koment` name and stronger checksum or attestation metadata.
3. The koment Claude marketplace and official Claude plugin directory package
   the strict instructions, hooks and MCP declaration. The official MCP
   Registry points at koment's labeled OCI artifact or a checksummed MCPB
   bundle rather than requiring an npm wrapper.
4. The VS Code Marketplace and Open VSX publish the same extension artifact
   when the editor integration exists.
5. Homebrew core, Nixpkgs, AUR, MacPorts and other community catalogs are
   pursued after a stable release where their external acceptance and ongoing
   maintenance requirements are met.

Release automation generates Homebrew, Scoop and WinGet metadata from one
version and checksum manifest and tests the directly controlled installation
channels. The repository publishes Claude marketplace metadata directly,
publishes the VSIX to VS Code Marketplace and Open VSX when their owner tokens
are configured, and uses GitHub OIDC for the MCP Registry. Owner-submitted
external catalogs are never described as available until their submissions are
accepted. ADR 0109 records the artifact and distribution boundary.

## Maintained workspace

The product carries a small, real repository workspace that builds, tests and
contains current annotations. It is a supported way to evaluate local CLI,
MCP, publishing and multi-repository behavior, not a gallery of intentionally
broken statuses and not a separate demo product.

Drifted, orphaned and ambiguous examples belong in deterministic testdata with
explicit before and after source pairs. Published project content never serves
known-stale rationale as though it were a maintained repository. The default
published route opens koment itself, and the maintained workspace is available
through the same contextual repository switcher as any other assigned
repository.

## Comment-free dogfooding

Before adding a comment, contributors rename, extract, introduce a named type or
constant, and restructure in that order. Remaining local rationale becomes a
koment annotation; structural rationale becomes an ADR.

Allowed inline comments are toolchain directives, links required to explain an
external constraint, deprecation markers and genuine public API documentation.
An AST-aware CI check enforces the Go rule. Other source-like files are audited
without blindly deleting schema directives or generated documentation markers.
ADR 0107 records why koment enforces its own thesis.

The policy guarantee is that an unacknowledged comment cannot pass `koment
comments check` and therefore cannot land through a protected branch. koment
cannot prevent an arbitrary editor or shell from placing bytes in an unsaved
buffer, and a local Git hook is bypassable. The required CI check is the
authoritative boundary; local hooks and editor diagnostics shorten feedback.

Comments fall into three classes:

1. Toolchain directives, generated-file markers, necessary upstream links,
   genuine API documentation and deprecation markers remain inline without an
   acknowledgement.
2. A newly written explanatory comment is comment intent. `koment comments
   convert` records its prose as an annotation before removing it from the
   source. The comment's placement selects a deterministic anchor in the code
   it described; the removed comment is never its own anchor. If conversion
   cannot choose a unique anchor, the source stays untouched and the operation
   fails loudly.
3. A comment that truly must stay inline requires `koment comments acknowledge`.
   CLI callers must pass `--acknowledge-inline-comment`; MCP callers must send
   `acknowledge_inline_comment: true`; editor callers confirm a prompt that
   names the rename, extraction, named-type and restructuring procedure. The
   result is a `why` annotation with the policy acknowledgement shown above.

There is no source-level `koment:ignore`, global CI bypass or path-wide escape
hatch for ordinary project code. Generated and vendored inputs are declared as
repository policy rather than waived ad hoc. Conversion writes the annotation
first and removes source prose second, so interruption may leave duplicate
rationale but cannot silently discard it.

## Agent contract and enforcement

A koment-enabled repository carries `.koment/policy.yaml`. Its format version
is 1 and it selects strict comment handling, intrinsic comment classes,
generated and vendored paths, and the agent adapters the repository supports.
This is the one machine-readable policy consumed by local checks, hooks and CI.
It cannot grant an ordinary source comment a path-wide exemption.

`koment agents install` makes that policy discoverable without asking every
contributor to copy prose from documentation. It installs or refreshes a
managed koment contract in root `AGENTS.md` and thin repository-owned adapters
for supported clients, including Copilot repository instructions, an
always-applied Cursor rule and client hooks where a trusted client exposes
them. Existing project-specific instructions outside the managed contract are
preserved. `koment agents check` fails if an installed adapter has drifted from
the policy or omits a mandatory rule.

Every adapter expresses the same strict contract:

1. Read `koment_get(file)` before editing an existing file and use
   `koment_search(query)` before changing a non-obvious decision.
2. Treat drifted, orphaned and ambiguous annotations as history, never current
   truth.
3. Do not add explanatory inline comments. Rename, extract, introduce a named
   type or constant, restructure, then use `koment_add`.
4. Convert completed comment intent through `koment_convert_comment`. Retain it
   only through `koment_acknowledge_comment` with the explicit acknowledgement
   and agent authorship.
5. Run `koment check`, `koment comments check` and `koment agents check` before
   completing work.

MCP initialization repeats the contract and exposes mutation tools in explicit
write mode, so an agent sees the procedure in the same session as the tools.
Where a client supports trusted repository hooks, a pre-write hook rejects
obvious newly added comments and a completion hook runs both checks and asks
the agent to continue until they pass. These hooks provide early feedback; they
are not a security boundary because clients can disable instructions, decline
repository trust or write through an unhooked tool.

The required `koment comments check` CI status is authoritative. It parses
supported languages, accepts only intrinsic comments or exact attributable
acknowledgements, and reports the command or MCP mutation that resolves each
failure. Branch protection makes an agent-produced prohibited comment unable
to land even when the agent ignored every instruction. An organization may add
managed client policy for stronger workstation enforcement, but koment does
not claim that repository files can control an arbitrary process.

## Implementation sequence

### 1. Operational floor — implemented

The pinned konflate-style toolchain, local hooks, the shared Renovate preset,
workflow audit, vulnerability scan and aggregate CI status are in place. The
existing race suite remains mandatory. Renovate runs on GitHub runners from
`.github/workflows/renovate.yml` (ADR 0122) and raises no pull request until a
GitHub App is installed on the repository, which is an account action rather
than a repository one; without it the workflow reports that it is inert.

### 2. Record and anchor reset — implemented

One-record-per-annotation storage, migrated records, the strict record schema,
contextual ambiguity resolution, rooted filesystem access, maintained workspace
content and removal of the local index/export subsystem are implemented and
verified together.

### 3. Shared reads and local writes — implemented

Introduce the snapshot and application services, move every reader to them,
surface provenance consistently, add local UI and MCP writes, and rebuild static
publishing and search. Add the shared comment-intent classifier, conversion,
explicit acknowledgement and policy check before editor-specific integration.
Add the version-1 repository policy, managed agent adapters, MCP initialization
instructions and adapter drift checks in the same stage so strict instructions
never advertise unavailable mutation tools.

### 4. Served multi-repository system — implemented

Add the unified server and authentication. Build a bounded GitHub synchronizer
that constructs immutable repository snapshots and search indexes away from
request handling, then swaps them atomically. Add the contextual repository
switcher and an idempotent GitHub materializer that returns success only after
the exact record exists in a branch and pull request. No database or durable
application outbox is part of this stage.

### 5. Deployment and release — implemented; external catalogs pending acceptance

Replace the prototype chart modes, add a values schema and E2E coverage, then
sign and digest-pin all release artifacts. Publish the canonical binaries,
container and chart before promoting their metadata through Homebrew, mise,
WinGet, Scoop, Claude and the MCP Registry. Publish the editor package to both
the VS Code Marketplace and Open VSX when stage 6 implements it.

### 6. Comment-intent and editor-native annotations — implemented

The editor-neutral `koment lsp` process is backed by the same repository
snapshot and mutation services. It exposes status diagnostics, hover content,
code lenses, native annotation views and add, reanchor, convert or acknowledge
commands without parsing or writing annotation records independently.

The thin VS Code extension starts koment, renders
annotation bodies beside their resolved source through native decorations and
gutter status, and opens focused editing controls when prose needs more space.
It never inserts virtual comments into the source buffer. A human or agent can
add and reanchor through the same application mutation contract, with explicit
authorship preserved.

In a koment-managed workspace, typing a syntactically complete explanatory
comment is the familiar entry path. The extension observes the completed edit,
asks the language-neutral service to classify it and, when the anchor is
unambiguous, replaces it with an annotation while preserving the inline visual
through a decoration. Intrinsic comments and text that appears to be temporarily
commented-out code are not converted. Uncertain cases remain in the buffer with
an immediate diagnostic and the actions `Convert to koment` and `Keep inline
with acknowledgement`.

The editor flow is acceleration, not enforcement. VS Code change events occur
after an edit, and save participants can be skipped or time-bounded. The
extension never races a save or deletes prose before the application service
has durably written its record. The same diagnostics and code actions are
available over LSP; native adapters add automatic conversion and richer
decorations where their editor APIs permit it.

Other editors receive the portable LSP surface first and may add native adapters
for richer inline presentation. Workspace-folder and repository ids define the
multi-repository boundary, so two repositories containing the same path never
share decorations or mutations accidentally.

The protocol remains useful without the VS Code package: editors can consume
the standard hover, diagnostic, code-lens, code-action and execute-command
surface directly. ADR 0110 records why mutation semantics remain in the Go
process instead of being duplicated in extensions.

## Definition of done

The approved design is complete when:

1. Concurrent local agents can add and reanchor records without losing work.
2. All four resolution statuses have real before/after fixtures and identical
   presentation across CLI, UI, MCP and static output.
3. No repository-controlled path or symlink can expose a file outside its root.
4. Humans and agents can read and write locally through first-class surfaces.
5. Static output is atomic, searchable, commit-stamped and machine-readable.
6. One authenticated service presents UI and MCP for several assigned
   repositories from atomically replaced, commit-stamped snapshots.
7. Remote writes retain exact content and author identity through a reviewed Git
   pull request.
8. The Helm E2E test installs the built image and passes health and functional
   probes in Kind.
9. Vulnerability, workflow, annotation, comment-policy and race checks pass in
   CI.
10. koment's own source has moved non-exempt rationale out of inline comments
    and its annotations resolve.
11. Typing an explanatory comment in the reference VS Code integration converts
    it to an attributed annotation, while an inline exception cannot pass CI
    without an exact, explicit acknowledgement record.
12. A fresh agent session in every supported client receives the same strict
    contract, can perform its required mutations and cannot land a prohibited
    comment when it ignores that contract.
13. Published and served multi-repository views open a useful default
    repository directly and switch repositories without a selector landing
    page.
14. A human can install koment on every supported operating system without a Go
    toolchain, and every advertised package, marketplace or registry entry is
    generated from an authenticated canonical release artifact.

## Non-goals

- LLM-generated annotations or semantic reanchoring.
- Consolidating, summarizing or expiring rationale as a memory system.
- Tree-sitter or language-specific symbol anchors before the deterministic record is in
  real use.
- A writable static site.
- Direct pushes from the served tier to a default branch.
- A full forge abstraction before the GitHub implementation proves the
  interface.
- An IDE plugin before the local and served application services are stable.
- Filesystem interception or an operating-system sandbox that claims to stop
  every process from writing comment bytes.

## Prior art

- [konflate](https://github.com/home-operations/konflate) — operational and
  deployment baseline.
- [Codetations / Magic Markup](https://github.com/elmisback/codetations) —
  document-external annotation research.
- [Robustly Anchoring Annotations Using Keywords](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/tr-2001-107.pdf)
  — deterministic anchoring prior art.
- [Architecture Decision Records](https://adr.github.io/) — project-wide
  decision history; koment records rationale local to code.
- [VS Code Extension API](https://code.visualstudio.com/api/references/vscode-api)
  — editor changes, diagnostics, code actions and decorations.
- [Language Server Protocol](https://microsoft.github.io/language-server-protocol/)
  — portable diagnostics and code actions across editors.
- [mise GitHub backend](https://mise.jdx.dev/dev-tools/backends/github.html) —
  direct installation of platform artifacts from GitHub Releases.
- [Homebrew taps](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap) —
  independently maintained binary distribution metadata.
- [Claude plugin marketplaces](https://code.claude.com/docs/en/plugin-marketplaces)
  — distribution for instructions, hooks, MCP and LSP integrations.
- [MCP Registry package types](https://modelcontextprotocol.io/registry/package-types)
  — OCI and MCPB-backed MCP discovery without a language wrapper.
