# koment

Keep the **why** next to your code without putting it *in* your code.

```
┌─ internal/store/ulid.go ──────────────────────────────────────┐
│ 18   paddingBits = ulidLength*bitsPerChar - 8*(...)  ●        │
│    ┌──────────────────────────────────────────────────┐       │
│    │ gotcha · ok                                      │       │
│    │ 26 Crockford characters carry 130 bits but a     │       │
│    │ ULID holds 128, so the value is left-padded by   │       │
│    │ two. Drop the padding and every character        │       │
│    │ shifts: the ids are still 26 characters of       │       │
│    │ valid alphabet, and still wrong.                 │       │
│    └──────────────────────────────────────────────────┘       │
└───────────────────────────────────────────────────────────────┘
```

Readable code answers *what*. It cannot answer *why this and not the obvious
alternative*, *what bit us here before*, or *what breaks if you "simplify" this*.

Comments are the usual home for that, and they rot. koment stores the reasoning
beside your code instead — in git, reviewable in a pull request, anchored to a
snippet rather than a line number, and **checked**. When the code an annotation
described changes, koment fails your build instead of quietly lying to you.

Your agents read it too. One MCP server serves Claude Code, Hermes, Cursor,
Codex and the rest, so the reasoning reaches whatever is editing your code.

---

## Install

```sh
go install github.com/janpuc/koment/cmd/koment@latest
```

Or build from a checkout — one static binary, no runtime:

```sh
git clone https://github.com/janpuc/koment && cd koment
go build -o koment ./cmd/koment
```

## Try it in two minutes

From inside any git repository:

```sh
# 1. record why something is the way it is
koment add src/auth.go \
    --excerpt 'if token.Expiry.Before(now.Add(-clockSkew)) {' \
    --kind gotcha \
    --body 'The skew subtraction is deliberate. Without it, clients whose clock
            runs a few seconds fast get logged out mid-request. Bit us in #412.'

# 2. read it back
koment show src/auth.go

# 3. gate on it
koment check          # exits non-zero if an annotation no longer resolves
```

Now edit that line and run `koment check` again. It fails — because the
reasoning you wrote no longer describes the code, and silently keeping it would
be exactly how comments rot.

Fix it without touching a file. The id comes straight from the `check` output:

```sh
koment reanchor 01KYW1ETE3CVB6S0ND70GGZVWM --excerpt 'if token.Expired(now) {'
```

## See it

```sh
koment ui
```

A local, read-only view: your files on the left with their drift status, your
code on the right with each annotation in the margin of the line it describes.
Drifted annotations show as struck-through history, never laid over current
code.

## Give it to your agents

An agent that cannot see why something was built a certain way will happily
refactor the reason away. koment ships an MCP server so it can.

```sh
koment mcp        # stdio
```

Pick your agent:

| | |
|---|---|
| [Claude Code](docs/agents/claude-code.md) | [Hermes](docs/agents/hermes.md) |
| [Cursor](docs/agents/cursor.md) | [OpenClaw](docs/agents/openclaw.md) |
| [VS Code](docs/agents/vscode.md) | [opencode](docs/agents/opencode.md) |
| [Zed](docs/agents/zed.md) | [Codex](docs/agents/codex.md) |
| [Anything else](docs/agents/other.md) | |

Your agent gets two tools: `koment_get(file)` for the file it is about to edit,
and `koment_search(query)` to find reasoning by topic. Every annotation arrives
with a resolution status, so a stale one is never presented as current.

## The four statuses

| | meaning | build |
|---|---|---|
| `ok` | the annotated code is exactly where it was | passes |
| `moved` | still there, shifted to a different line | passes |
| `drifted` | the file exists, the annotated code is gone | **fails** |
| `orphaned` | the file is gone | **fails** |

Drift is a failure, not a warning. That is the entire point: an annotation
nobody revisited is worse than no annotation at all.

## The four kinds

Constrained deliberately, so it does not become a junk drawer.

- `why` — why this and not the obvious alternative
- `gotcha` — what bit someone here before
- `invariant` — what must stay true
- `anti-pattern` — what was tried and does not work

## Where it lives

```
your-repo/
├── .koment/annotations/src/auth.go.yaml    ← committed, reviewable
└── src/auth.go
```

One file per annotated source file, in git. Diffs stay local, annotation changes
show up in the same pull request as the code that motivated them, and nothing
lives on a server you have to keep running.

## Put it in CI

```yaml
- run: koment check
```

## Documentation

- **[Getting started](docs/quickstart.md)** — the full walkthrough
- **[Writing good annotations](docs/annotating.md)** — what earns one, what doesn't
- **[Agent setup](docs/agents/)** — per-client configuration
- **[CI and pre-commit](docs/ci.md)** — wiring the drift gate
- **[CLI reference](docs/cli.md)** — every command and flag

Contributing or curious how it works? **[docs/development.md](docs/development.md)**
covers the internals, and **[docs/decisions/](docs/decisions/)** records why koment
is the way it is — including the roads not taken.

## What koment is not

- **Not a replacement for ADRs.** Project-wide decisions belong in a decision
  record. koment holds knowledge bound to a *place* in the code.
- **Not a memory system.** A consolidating store paraphrases, merges and
  eventually deletes. koment is a record, not a belief. ([ADR 0004](docs/decisions/0004-do-not-use-a-memory-store-as-the-backend.md))
- **Not line-precise.** Anchors are snippets, not line numbers. ([ADR 0003](docs/decisions/0003-anchor-by-excerpt-not-line-numbers.md))

## Prior art

[Codetations / Magic Markup](https://github.com/elmisback/codetations) is the
closest existing work — document-external annotations with LLM re-anchoring —
and validates the approach, though it remains a research prototype. The
anchoring problem itself dates to [Microsoft Research, 2001](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/tr-2001-107.pdf)
and is still unsolved in the general case. [ADRs](https://adr.github.io/) cover
the project-wide half of the problem and koment deliberately does not compete
with them.

## License

MIT
