<div align="center">

<img src="internal/ui/assets/koment-logo.svg" alt="koment comment bubble" width="104">

# koment

**Keep the _why_ next to your code — checked, so it can't quietly rot.**

[![CI](https://img.shields.io/github/actions/workflow/status/janpuc/koment/ci.yml?branch=main&label=ci)](https://github.com/janpuc/koment/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/actions/workflow/status/janpuc/koment/release.yml?branch=main&label=release)](https://github.com/janpuc/koment/actions/workflows/release.yml)
[![Annotations](https://img.shields.io/badge/annotations-browse-brightgreen)](https://janpuc.github.io/koment/)
[![License](https://img.shields.io/github/license/janpuc/koment)](https://github.com/janpuc/koment/blob/main/LICENSE)

</div>

> **Design reset in progress.** This README documents the runnable v0.2
> implementation. The approved breaking vNext target and its honest capability
> status are in [DESIGN.md](DESIGN.md); active decisions begin at
> [ADR 0100](docs/decisions/README.md).

Readable code answers *what*. It cannot answer *why this and not the obvious
alternative*, *what bit us here before*, or *what breaks if you "simplify" this*.
Comments are the usual home for that and they rot — they duplicate the code,
drift from it silently, and nothing ever tells you they have stopped being true.

koment puts that reasoning **beside** the code instead: prose anchored to a
verbatim snippet, stored in `.koment/` in git, reviewable in the same pull
request as the change that motivated it. Then it **checks** the anchor. When the
annotated code changes, `koment check` fails the build rather than serving a note
that no longer describes anything.

Your agents read it too. One MCP server gives Claude Code, Cursor, Codex, Zed,
Hermes and the rest the same reasoning through the same interface — because an
agent that cannot see why something was built a certain way will happily
refactor the reason away.

**[See it running →](https://janpuc.github.io/koment/)** — koment's own
annotations, rendered by koment onto GitHub Pages by [the workflow you can
copy](docs/publishing.md).

## How it works

1. You annotate a snippet. koment records the prose, the excerpt and its
   SHA-256, the commit you were on, and who you are.
2. The record lands in `.koment/annotations/<mirrored path>.yaml`, one file per
   annotated source file. It is YAML, it is in git, and it diffs line by line.
3. Resolution searches the current file for that excerpt and produces exactly
   one status:

   | | meaning | build |
   |---|---|---|
   | `ok` | found where it was last seen | passes |
   | `moved` | still there, different line | passes |
   | `drifted` | file exists, the annotated code is gone | **fails** |
   | `orphaned` | the file is gone | **fails** |

4. `koment check` exits non-zero on `drifted` or `orphaned`. That is the whole
   mechanism: an annotation nobody revisited is worse than no annotation, so it
   has to be impossible to ignore.
5. When it fails, `koment reanchor <id> --excerpt '<new text>'` repoints it —
   keeping its id and creation date, recomputing the hash and line for you.
   Nothing re-attaches automatically; a person confirms the reasoning still holds.

Anchoring is by **excerpt**, never by line number — line numbers rot on the next
edit above them. The commit hash *is* recorded, but only to reconstruct history;
it never decides whether an annotation still applies. Two questions, two
mechanisms ([ADR 0100](docs/decisions/0100-one-git-record-per-annotation.md)).

koment is **read-only over the network**. The UI and the MCP server serve; they
never write. Annotations are created by the CLI, by a person or an agent with a
checkout — so a published instance exposes no way for a visitor to change
anything.

## Three ways to run it

Pick one. Each is a place to stop, not a step you have to take, and **moving
between them is not a migration** — all three read the same `.koment/` in git,
so there is nothing to export, import or back up
([ADR 0103](docs/decisions/0103-three-tiers-with-human-and-agent-capabilities.md)).

| | you run | you get |
|---|---|---|
| **local** | the CLI, and `koment mcp` in your agent's config | annotate, `koment check`, agents read the reasoning. Nothing to host. |
| **published** | [one workflow file](docs/publishing.md) → GitHub Pages | everyone reads the annotations in a browser. No server, no auth to design, no cost. |
| **served** | the container or the [Helm chart](#kubernetes) | the live working tree, several repositories, search across them, metrics |

## Quick start

```bash
go install github.com/janpuc/koment/cmd/koment@latest
```

No Go toolchain? Every release carries binaries for linux, macOS and Windows on
amd64 and arm64, with checksums —
[grab one](https://github.com/janpuc/koment/releases/latest).

```bash
cd ~/your-project
koment add src/auth.go \
  --excerpt 'if token.Expiry.Before(now.Add(-clockSkew)) {' \
  --kind gotcha \
  --body 'The skew subtraction is deliberate. Without it, clients whose clock
          runs fast get logged out mid-request. Bit us in #412.'

koment check   # green
koment ui      # look at it
```

Now edit that line and run `koment check` again. It fails — because the reasoning
you wrote no longer describes the code, and silently keeping it is exactly how
comments rot.

Or run the viewer against a checkout with the container:

```bash
docker run --rm -p 8080:8080 -v "$PWD:/repo:ro" ghcr.io/janpuc/koment:latest
```

## Several repositories

One deployment serves many. Identity is assigned, so a repository that moves
keeps its annotations and its index
([ADR 0104](docs/decisions/0104-transactional-multi-repository-snapshots.md)):

```yaml
# KOMENT_CONFIG=/etc/koment.yaml
repositories:
  - id: payments          # assigned once; never derived from the path
    name: Payments API
    root: /repos/payments
    clone_url: https://github.com/you/payments
    default_branch: main
  - id: web
    root: /repos/web
```

Or, for a container with a couple of mounts and nothing else to say:

```bash
KOMENT_REPOS=payments=/repos/payments,web=/repos/web
```

A single repository needs none of this — koment finds it by walking up from the
working directory, exactly as before.

## Kubernetes

koment publishes an **OCI** Helm chart to `oci://ghcr.io/janpuc/charts/koment`:

```bash
helm install koment oci://ghcr.io/janpuc/charts/koment \
  --set repository.clone.enabled=true \
  --set repository.clone.url=https://github.com/you/your-repo \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.dashboard.enabled=true
```

Metrics land on a **separate port** from the serving one — the serving port is
unauthenticated and ingress-facing, and `/metrics` must not inherit that
([ADR 0103](docs/decisions/0103-three-tiers-with-human-and-agent-capabilities.md)). The
chart ships a Grafana dashboard the sidecar discovers by label. Its headline
panel is drift over time, which is the one question `koment check` cannot answer,
because it only ever sees one moment.

## Configuration

Every flag can be set from the environment, which is how you configure the
container. `--flag-name` becomes `KOMENT_FLAG_NAME`, and an explicit flag always
wins.

| | |
|---|---|
| `KOMENT_REPO` | one repository; defaults to the working directory |
| `KOMENT_REPOS` | several: `api=/repos/api,web=/repos/web` |
| `KOMENT_CONFIG` | a YAML registry, for per-repository settings |
| `KOMENT_LISTEN` | address for `koment ui` |
| `KOMENT_HTTP` | address for `koment mcp --http` |
| `KOMENT_STREAMABLE_HTTP` | address for `koment mcp --streamable-http` |
| `KOMENT_METRICS` | address for the metrics listener; off unless set |
| `KOMENT_OUT` | output directory for `koment export` |
| `KOMENT_INDEX` | SQLite index file; defaults to the cache directory |
| `KOMENT_DATABASE_URL` | use Postgres for the index, which makes the service stateless |

`.koment/` bootstraps the index when it is empty, and `koment export` rebuilds
`.koment/` from the index — byte-identically. Either can be lost without losing
annotations.

`koment <command> --help` lists every flag alongside its variable.

## Give it to your agents

```bash
koment mcp        # stdio; also --http / --streamable-http for remote agents
```

Three tools:

- **`koment_get(file, repository?)`** — annotations for the file an agent is about to edit
- **`koment_search(query, repository?)`** — find reasoning by topic; omitting `repository` searches all of them
- **`koment_repositories()`** — what this deployment serves, with counts

Every annotation arrives with its resolution status *and* its repository, so a
stale one is never presented as current and a result is never detached from its
scope. When a path exists in several repositories `koment_get` **refuses and
names the candidates** rather than guessing
([ADR 0104](docs/decisions/0104-transactional-multi-repository-snapshots.md)).

| | | |
|---|---|---|
| [Claude Code](docs/agents/claude-code.md) | [Cursor](docs/agents/cursor.md) | [VS Code](docs/agents/vscode.md) |
| [Zed](docs/agents/zed.md) | [Codex](docs/agents/codex.md) | [opencode](docs/agents/opencode.md) |
| [Hermes](docs/agents/hermes.md) | [OpenClaw](docs/agents/openclaw.md) | [Anything else](docs/agents/other.md) |

## Publish it

Your reviewers are not going to install anything. Give them a URL: one workflow
file renders every annotation to static HTML and puts it on GitHub Pages, with
no server, no database and no authentication to design.

```yaml
- uses: actions/checkout@v5
- uses: janpuc/koment@v0.2.0
- run: koment check
- run: koment site --out dist
```

Every page names the commit it was rendered from, so a snapshot can never pass
for the current tree. `koment check` in the same workflow means a build that
would publish drift fails first.

**[The whole workflow, ready to copy →](docs/publishing.md)**

A site renders your source as well as your annotations, so publishing one from a
private repository publishes that source. One repository per site, by design —
a static page has no server to resolve a repository against, and faking one
would be worse than the limit.

## Documentation

- **[Publishing](docs/publishing.md)** — the copy-paste workflow, and moving to a served instance later
- **[Bootstrap](docs/bootstrap.md)** — what it is, the data model, running it, releases
- **[Getting started](docs/quickstart.md)** · **[Writing good annotations](docs/annotating.md)** · **[CLI reference](docs/cli.md)** · **[CI](docs/ci.md)**
- **[Decisions](docs/decisions/)** — why it is built this way, and what was rejected

## What koment is not

- **Not a replacement for ADRs.** Project-wide decisions belong in a decision
  record; koment holds knowledge bound to a *place* in the code.
- **Not a memory system.** A consolidating store paraphrases, merges and
  eventually forgets. koment is a record, not a belief
  ([ADR 0100](docs/decisions/0100-one-git-record-per-annotation.md)).
- **Not line-precise.** Anchors are snippets
  ([ADR 0101](docs/decisions/0101-fail-ambiguous-anchor-resolution.md)).

## Prior art

[Codetations / Magic Markup](https://github.com/elmisback/codetations) is the
closest existing work — document-external annotations with LLM re-anchoring. The
anchoring problem dates to [Microsoft Research, 2001](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/tr-2001-107.pdf)
and is still unsolved in the general case. The UI owes its shape to
[konflate](https://github.com/home-operations/konflate), which makes an
invisible layer visible for Flux the way koment tries to for rationale.

## License

MIT
