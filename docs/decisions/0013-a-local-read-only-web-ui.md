# 0013 — A local, read-only web UI where code and annotations converge

Date: 2026-07-31
Status: Accepted

## Context

Everything koment ships so far is built for machines. `koment_get` hands an
agent structured JSON; `check` returns an exit code; `show` prints a list of
annotations *next to* a file rather than *against* it. A person who wants to
understand a file has to hold two things in their head at once — the source in
their editor and a list of bodies in a terminal — and do the joining themselves.

That joining is the entire value. An annotation is meaningless away from the
line it describes. The CLI's output is a report about the code; what a person
needs is the code, with the reasoning attached at the point it applies.

Prior art, and the direct inspiration: **home-operations/konflate** — "a
read-only pull request review tool for Flux". Its thesis is structurally the
same as koment's. A raw file diff shows a one-line chart bump; konflate renders
the cluster before and after and shows what actually changed. The raw source
shows what the code does; koment holds why. Both are tools that make an
invisible layer visible, and both are read-only by design.

Inspected at commit time: konflate is a Go backend (79 `.go` files) with a
Svelte frontend (19 `.svelte`, 20 `.ts`) built to `internal/web/dist/` and
embedded through `internal/web/embed.go`, so the shipped artifact stays a single
Go binary. Its layout is three panels — PR list, resource tree, diffs with
word-level highlighting — plus status pills, a command palette, and websocket
updates as renders complete.

Two things there are worth taking, and one is worth leaving.

## Decision

Add `koment ui`, a local read-only web view.

```
koment ui [--listen <addr>]     # default 127.0.0.1, port chosen and printed
```

**Layout — two panels, not three.** konflate needs three because its hierarchy
is PR → resource → diff. koment's is file → code, and the annotations belong
*on* the code rather than in a list beside it. Collapsing to two panels is what
makes the convergence visible instead of merely adjacent.

```
┌──────────────────┬────────────────────────────────────────────────┐
│ 13 annotations   │  internal/store/ulid.go                        │
│ 10 ok  3 moved   │                                                │
│                  │   16   const (                                 │
│ internal/        │   17     crockfordAlphabet = "0123456789AB…"   │
│  store/          │ ▎ 18     paddingBits = ulidLength*bitsPer…   ●  │
│   ulid.go    ● 1 │        ┌────────────────────────────────────┐  │
│   store.go   ● 2 │        │ gotcha · ok · 2026-07-31           │  │
│   record.go  ● 1 │        │ 26 Crockford characters carry 130  │  │
│  cli/            │        │ bits but a ULID holds 128, so the  │  │
│   cli.go     ● 2 │        │ value is left-padded by two…       │  │
│   add.go     ● 1 │        └────────────────────────────────────┘  │
│   reanchor.go● 2 │   19     timestampBytes = 6                    │
│  mcp/            │   20   )                                       │
│   mcp.go     ● 1 │                                                │
└──────────────────┴────────────────────────────────────────────────┘
```

**Status is the visual primary.** Every file in the tree carries its worst
status, and every annotation card is coloured by it. A `drifted` annotation is
rendered as struck-through history with its excerpt shown as *what used to be
there*, never laid over current code as though it still described it. This is
ADR 0005's guarantee expressed visually rather than as a `warning` string.

**Server-rendered Go templates, `go:embed`, no npm.** This is where koment
parts company with konflate. Templates in `internal/ui`, CSS and a small amount
of vanilla JavaScript embedded into the binary. No node toolchain, no build
step, no `package.json`, and no new Go module.

**Read-only, loopback, unauthenticated** — the same posture as ADR 0011, for
the same reason and with the same condition attached.

## Consequences

- The thing a person actually needs — code and reasoning in one view — exists
  for the first time. Everything before this was addressed to machines.
- Adopting konflate's embedding trick while rejecting its frontend stack keeps
  the single-binary property with zero new dependencies. `koment ui` works from
  a checkout with nothing installed but Go.
- Server rendering means no live updates. Editing a file requires a refresh to
  see new drift. konflate needs websockets because its renders complete
  asynchronously; koment reads local files in milliseconds, so a refresh is an
  honest trade for an entire toolchain.
- Hand-written templates and CSS will be less polished than a component
  framework. Accepted: the surface is two panels and a card, not an application.
- **No syntax highlighting in the first version.** Doing it properly needs a
  lexer per language, and the obvious candidate (`chroma`) is a large
  dependency that must win its own ADR. Code renders as plain monospace, which
  is legible and honest.
- Read-only means seeing drift in the UI and then fixing it in a terminal. That
  is a real seam, and the obvious next feature is a re-anchor button — which is
  a write path, and **must not ship without revisiting the auth posture**, per
  the condition ADR 0011 attached to itself.
- A fourth surface to keep working. Mitigated by rendering from the same
  `anchor.Resolve` results the CLI and MCP server use; the UI adds a view, not
  a second source of truth.

## Alternatives rejected

- **Svelte or React, embedded like konflate's.** The proven approach, in the
  exact repository being copied from, and it would look better. Rejected
  because it imports an entire second ecosystem — node, npm, a lockfile, a
  build step, a committed `dist/` — into a project whose Go graph is two direct
  modules and whose AGENTS.md sets a deliberately high dependency bar. konflate
  earns it with live websocket updates and word-level diffing; koment's view is
  a file and some cards. Revisit if the UI genuinely outgrows templates.
- **An editor extension instead** (VS Code, JetBrains). Better placement —
  annotations would sit in the editor where the work happens. Rejected as the
  starting point for the reason ADR 0005 rejected per-client hooks: one
  implementation per editor, and it strands anyone using a different one. A
  local web view serves every editor and every browser at once. This remains the
  right *eventual* answer for authoring.
- **A static site generator — `koment export --html`.** No server, publishable
  as an artifact, trivially cacheable. Rejected because it goes stale the moment
  the code changes, which is the failure mode koment exists to prevent. Serving
  live from the working tree means what you see is what is on disk.
- **Rendering annotations into the source as comments for display.** The most
  literal reading of "make them converge". Rejected outright: it is the thing
  the project refuses to do, and a reader would inevitably copy one back into a
  file.
- **Extending the MCP server with a UI-shaped tool.** Reuses the transport.
  Rejected because ADR 0005 fixed the tool surface at two on a measured context
  budget, and a browser is not an MCP client.
- **Three panels, mirroring konflate exactly.** Rejected as cargo-culting: the
  middle panel would list annotations for the selected file, which is precisely
  the separation between code and reasoning this ADR exists to remove.
