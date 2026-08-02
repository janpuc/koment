<div align="center">

# koment

**Keep the _why_ next to your code — checked, so it can't quietly rot.**

[![CI](https://img.shields.io/github/actions/workflow/status/janpuc/koment/ci.yml?branch=main&label=ci)](https://github.com/janpuc/koment/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/actions/workflow/status/janpuc/koment/release.yml?branch=main&label=release)](https://github.com/janpuc/koment/actions/workflows/release.yml)
[![Demo](https://img.shields.io/badge/demo-live-brightgreen)](https://janpuc.github.io/koment/)
[![License](https://img.shields.io/github/license/janpuc/koment)](https://github.com/janpuc/koment/blob/main/LICENSE)

</div>

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

**[See it running →](https://janpuc.github.io/koment/)** — that is koment's own
annotations, rendered by koment.

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
mechanisms ([ADR 0014](docs/decisions/0014-record-the-git-context-of-every-annotation.md)).

koment is **read-only over the network**. The UI and the MCP server serve; they
never write. Annotations are created by the CLI, by a person or an agent with a
checkout — so a published instance exposes no way for a visitor to change
anything.

## Quick start

```bash
go install github.com/janpuc/koment/cmd/koment@latest

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
([ADR 0020](docs/decisions/0020-expose-metrics-on-a-separate-listener.md)). The
chart ships a Grafana dashboard the sidecar discovers by label. Its headline
panel is drift over time, which is the one question `koment check` cannot answer,
because it only ever sees one moment.

## Configuration

Every flag can be set from the environment, which is how you configure the
container. `--flag-name` becomes `KOMENT_FLAG_NAME`, and an explicit flag always
wins.

| | |
|---|---|
| `KOMENT_REPO` | repository to serve; defaults to the working directory |
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

Two tools: `koment_get(file)` for the file an agent is about to edit, and
`koment_search(query)` to find reasoning by topic. Every annotation arrives with
its resolution status, so a stale one is never presented as current.

| | | |
|---|---|---|
| [Claude Code](docs/agents/claude-code.md) | [Cursor](docs/agents/cursor.md) | [VS Code](docs/agents/vscode.md) |
| [Zed](docs/agents/zed.md) | [Codex](docs/agents/codex.md) | [opencode](docs/agents/opencode.md) |
| [Hermes](docs/agents/hermes.md) | [OpenClaw](docs/agents/openclaw.md) | [Anything else](docs/agents/other.md) |

## CI

```yaml
- run: koment check
```

## Documentation

- **[Bootstrap](docs/bootstrap.md)** — what it is, the data model, running it, releases
- **[Getting started](docs/quickstart.md)** · **[Writing good annotations](docs/annotating.md)** · **[CLI reference](docs/cli.md)** · **[CI](docs/ci.md)**
- **[Decisions](docs/decisions/)** — why it is built this way, and what was rejected

## What koment is not

- **Not a replacement for ADRs.** Project-wide decisions belong in a decision
  record; koment holds knowledge bound to a *place* in the code.
- **Not a memory system.** A consolidating store paraphrases, merges and
  eventually forgets. koment is a record, not a belief
  ([ADR 0004](docs/decisions/0004-do-not-use-a-memory-store-as-the-backend.md)).
- **Not line-precise.** Anchors are snippets
  ([ADR 0003](docs/decisions/0003-anchor-by-excerpt-not-line-numbers.md)).

## Prior art

[Codetations / Magic Markup](https://github.com/elmisback/codetations) is the
closest existing work — document-external annotations with LLM re-anchoring. The
anchoring problem dates to [Microsoft Research, 2001](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/tr-2001-107.pdf)
and is still unsolved in the general case. The UI owes its shape to
[konflate](https://github.com/home-operations/konflate), which makes an
invisible layer visible for Flux the way koment tries to for rationale.

## License

MIT
