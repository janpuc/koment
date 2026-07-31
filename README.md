# koment

Out-of-band code annotations for AI agents. Keeps the **why** next to the code
without putting it *in* the code.

> Status: **v1 implemented.** Start at `DESIGN.md`, then `docs/decisions/`. If
> you are an agent, `AGENTS.md` is mandatory reading.

## The problem

Readable code answers *what*. It cannot answer *why this and not the obvious
alternative*, *what bit us here before*, or *what invariant breaks if you
"simplify" this*.

Comments are the usual home for that, and they rot: they duplicate the code,
drift from it, carry no history, and cannot reference anything outside the file.

Meanwhile several agents work across the same codebase. An agent that cannot see
why something was built a certain way will happily refactor the reason away.

## The approach

Annotations live in `.koment/`, committed to the repo, anchored to a verbatim
excerpt rather than a line number. Every anchor resolves to a status:

```
ok        excerpt found, hash matches
moved     excerpt found at a different offset
drifted   file exists, excerpt gone        → non-zero exit
orphaned  file gone                        → non-zero exit
```

Drift is a **failure**, not a warning. The annotated code changed and nobody
revisited the annotation — which is precisely how comments rot. koment's job is
to make that impossible to ignore.

Agents read annotations through an MCP server, so one implementation serves
Claude Code, Hermes, opencode and Codex alike.

## Use it

```sh
go build -o koment ./cmd/koment

koment add internal/store/store.go \
    --excerpt 'func writeAtomically(path string, content []byte) error {' \
    --kind why --body 'A truncated record is indistinguishable from a corrupt one.'

koment show internal/store/store.go   # resolved annotations for one file
koment list --kind gotcha             # everything, filtered
koment check                          # drift gate; non-zero on drifted/orphaned
```

## Wire it into an agent

`.mcp.json` is committed, so an MCP-capable client working in this checkout
picks up `koment_get` and `koment_search` with no setup. For another repository,
drop in the same file:

```json
{ "mcpServers": { "koment": { "command": "koment", "args": ["mcp"] } } }
```

stdio is the default. Agents that cannot spawn a subprocess can use HTTP
instead — loopback unless you say otherwise, and unauthenticated either way
(ADR 0011):

```sh
koment mcp --http 8765              # JSON responses
koment mcp --streamable-http 8765   # server-sent events
```

Put `koment check` in CI. Drift is a build failure, which is the entire point.

## What koment is not

- Not a replacement for ADRs. Project-wide decisions belong in
  `docs/decisions/`; koment holds knowledge bound to a *place* in the code.
- Not a memory system. A consolidating store paraphrases, merges and eventually
  deletes — see ADR 0004 for the measured evidence.
- Not line-precise. See ADR 0003.

## Layout

```
DESIGN.md            architecture, data model, CLI/MCP surface, roadmap
AGENTS.md            rules for agents working on this repo
docs/decisions/      ADRs — why it is the way it is
.koment/             koment's annotations about koment (ADR 0010)
.mcp.json            MCP wiring, committed so agents need no setup
cmd/koment/          entrypoint, flag parsing only
internal/store/      read/write .koment/annotations
internal/anchor/     resolution and drift status
internal/cli/        add, show, check, list
internal/mcp/        MCP server, stdio and HTTP
```

## Prior art

[Codetations / Magic Markup](https://github.com/elmisback/codetations) is the
closest existing work — document-external annotations with LLM re-anchoring — and
validates the approach, though it remains a research prototype. The anchoring
problem itself dates to [Microsoft Research, 2001](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/tr-2001-107.pdf)
and is still unsolved in the general case. [ADRs](https://adr.github.io/) cover
the project-wide half of the problem and koment deliberately does not compete
with them.
