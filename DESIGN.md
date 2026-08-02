# koment — design

Status: **implemented.** Change it by proposing a diff, not by diverging in
code; the decisions behind it are in `docs/decisions/`.

## Problem

Code should be readable enough not to need comments. But readable code only
answers **what**. It cannot answer **why this and not the obvious alternative**,
**what bit us here before**, or **what invariant will silently break if you
"simplify" this**.

Multiple agents work across these codebases. An agent that cannot see why
something was built a certain way will happily refactor the reason away.

Inline comments are rejected: they duplicate *what*, drift from the code, and
have no history, no confidence, and no cross-file reach.

## Non-goals

- Replacing documentation, READMEs or ADRs. koment holds **local** knowledge
  bound to a place in the code. Project-wide decisions belong in
  `docs/decisions/`.
- Being a memory system. See ADR 0004 — a consolidating memory store will
  paraphrase, merge and eventually delete annotations. koment is a **record**,
  not a belief.
- Line-precise annotation. See ADR 0003.

## Shape

An annotation store that lives **beside** the code, in git, with anchors that
are checkable. Stale annotations are detected and reported loudly, never served
silently.

```
repo/
├── .koment/
│   └── annotations/
│       └── internal/anchor/resolve.go.yaml
├── internal/anchor/resolve.go
└── docs/decisions/
```

One annotation file mirrors one source file. Rationale in ADR 0002.

### Record format

```yaml
version: 1
file: internal/anchor/resolve.go
annotations:
  - id: 01JQ8ZK3M4N5P6R7S8T9V0W1X2
    scope: excerpt
    excerpt: "if a.Excerpt == \"\" {"
    excerpt_sha256: 3f2a...
    last_seen_line: 42
    kind: gotcha
    body: >
      An empty excerpt means file-scope, not "matches everything". Treating it
      as a wildcard made every annotation resolve to the first line.
    created: 2026-07-31
```

- `id` — ULID. Stable across edits; never reused.
- `scope` — `file` or `excerpt` in v1. `symbol` is deferred (ADR 0003).
- `last_seen_line` — where the excerpt was found last time, 1-based. A **hint**,
  never an anchor: resolution ignores it when searching and consults it only to
  tell `ok` from `moved`. Losing it costs a status distinction, never a match.
  ADR 0009.
- `kind` — `why` | `gotcha` | `invariant` | `anti-pattern`. Constrained on
  purpose; a free-form kind field becomes a junk drawer.
- `body` — prose. The thing that would have been a comment.
- `git` — the context at creation. Written once, never rewritten. ADR 0014.
- `author` — who wrote it, and how much that claim is worth. ADR 0015.

```yaml
    git:
      commit: 9f3c1a4d8e2b7c5a6f0d3e1b8c7a5f2d4e6b9c1a
      path: internal/store/ulid.go
      line: 18
      end_line: 18
    author:
      name: Jan Pucilowski
      email: janpuc@proton.me
      kind: human
      source: git-config
```

### Two jobs, two mechanisms

The excerpt and the git context answer different questions, and neither
substitutes for the other:

| | question | mechanism | changes |
|---|---|---|---|
| anchor | does this still apply *now*? | excerpt search | every resolution |
| git context | what was true when it was written? | commit hash | never |

Resolution reads the excerpt and nothing else. Deleting the whole `git` block
changes no status — there is a test for that. The commit hash is authoritative
for reconstructing history; the excerpt is authoritative for applicability.

## Data model

A deployment may serve many repositories. The repository stays the unit of
storage — each keeps its own `.koment/` — and the deployment indexes them
rather than owning them. ADR 0017.

```
Deployment
└── Repository        id, name, clone URL, default branch, config, sync state
    └── Commit        the git context an annotation was created against
        └── File      path at that commit
            └── Annotation
```

Isolation is structural: a repository's annotations live in that repository, so
there is no shared table where a missing filter leaks one project into another.

### Lifecycle

Annotations may be created interactively, but the repository is the record and
the deployment store is an outbox, never a mirror. ADR 0016.

```
created ──▶ pending ──▶ materialised ──▶ settled
            (in the deployment,          (in git, authoritative;
             not in any clone)            the outbox keeps no copy)
```

On conflict git wins, because after materialising there is nothing left in the
outbox to conflict with. The ULID is minted at creation and survives, so pending
and settled are the same annotation.

**Interactive creation is a write path.** ADR 0011 made its own
no-authentication posture conditional on the surface staying read-only. Nothing
that creates an annotation may be exposed on a network listener until
authentication exists.

### Anchor resolution

Every anchor resolves to exactly one status:

| status | meaning | exit |
|---|---|---|
| `ok` | file exists; excerpt found at `last_seen_line` | 0 |
| `moved` | excerpt found, but at a different line | 0 |
| `drifted` | file exists, excerpt no longer found | non-zero |
| `orphaned` | file gone | non-zero |

`drifted` and `orphaned` are **failures**, not warnings. The annotated code
changed and nobody revisited the annotation, which is precisely how comments rot.
Forcing a human or agent to re-confirm is the entire value proposition.

## Surface

```
koment add <file> [--excerpt <text>] --kind <kind> --body <text>
koment show <file>              # annotations for one file, resolved
koment check [path...]          # drift gate; non-zero on drifted/orphaned
koment list [--kind k]          # everything, for review
koment reanchor <id> [--excerpt <text>] [--file <path>]   # fix drift; keeps the id
koment mcp                      # MCP server over stdio
koment mcp --http <addr>              # ... over HTTP, JSON responses
koment mcp --streamable-http <addr>   # ... over HTTP, SSE responses
```

stdio remains the default and the recommended transport. The HTTP transports
exist for agents that cannot spawn a subprocess — a container, a remote runner —
and bind to loopback unless told otherwise. ADR 0011.

`koment mcp` is the reason this project is worth building rather than adopting a
convention. One server, every agent — Claude Code, Hermes, opencode, Codex —
gets the same annotations through the same interface, with no per-client
plumbing.

MCP tools exposed:

- `koment_get(file)` — annotations for a path, with resolution status
- `koment_search(query)` — full-text across bodies

### `koment ui`

```
koment ui [--listen <addr>]     # local read-only web view, loopback default
```

Everything above is addressed to machines. `show` prints annotations *beside* a
file; a person needs them *on* it. `koment ui` is two panels — a file tree
carrying drift status, and the source with annotation cards anchored in the
margin of the line they describe — rendered from Go templates embedded with
`go:embed`, no node toolchain and no new dependency. ADR 0013.

Every request re-reads the working tree, so what is rendered is what is on
disk. A `drifted` annotation is shown as struck-through history and never laid
over current code.


Prior art: [konflate](https://github.com/home-operations/konflate), a read-only
Flux PR review tool with the same shape of thesis. koment takes its
embed-the-frontend-in-the-binary trick and its status-first visual language, and
declines its Svelte build.

## Implementation

Go. Single static binary, no runtime, trivial to ship into a container or onto a
laptop. Consistent with the rest of this stack.

```
cmd/koment/          entrypoint, flag parsing only
internal/cli/        the add/show/check/list/reanchor commands
internal/store/      read/write .koment/annotations
internal/anchor/     resolution and drift status
internal/listen/     bind address resolution, shared by both servers
internal/mcp/        MCP server
internal/ui/         local read-only web view
```

Standard library wherever possible. Every dependency needs an ADR.

## Definition of done for v1

1. `store` reads and writes the record format, round-trips losslessly.
2. `anchor` resolves all four statuses, with tests per status.
3. `koment check` exits non-zero on drift.
4. `koment show` prints resolved annotations for a file.
5. `koment mcp` serves `koment_get` and `koment_search` over stdio and HTTP.
6. koment's own `.koment/` holds real annotations about koment, and every agent
   working in this repository reaches them without being told how (ADR 0010).

Point 6 is not decoration. If the tool is not useful on itself, it is not
useful.

## Deferred

Do not start these until v1 is in real use:

- **Symbol scope** via tree-sitter. Survives moves and reformatting; costs a
  large dependency and per-language grammars.
- **LLM re-anchoring** — Codetations/Magic Markup style semantic re-attachment
  when an excerpt drifts. This is the interesting problem and the reason to
  build the deterministic layer first: without it there is no ground truth to
  evaluate re-anchoring against.
- Editor integration and annotation suggestion.

*(`koment reanchor` and the web UI were on this list and are now built —
ADR 0012 and ADR 0013.)*

## Prior art

- [Codetations / Magic Markup](https://github.com/elmisback/codetations) —
  document-external annotations with LLM re-anchoring. Research prototype;
  validates the approach, not production-ready.
- [Microsoft Research, *Robustly Anchoring Annotations Using Keywords*](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/tr-2001-107.pdf)
  — the anchoring problem, 2001. Still unsolved in the general case.
- [ADRs](https://adr.github.io/) — the right home for project-wide decisions.
  koment is deliberately the *local* complement, not a competitor.
