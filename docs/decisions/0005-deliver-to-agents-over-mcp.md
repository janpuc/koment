# 0005 — Deliver to agents over MCP

Date: 2026-07-31
Status: Accepted; the transport decision is amended by 0011

## Context

Annotations are worthless unless an agent actually sees them at the moment it
touches the code. Several different agents work across these repositories —
Claude Code, Hermes, opencode, Codex — each with its own configuration surface,
hook system and instruction-file convention.

Writing and maintaining a bespoke integration per client is how this project
dies of maintenance.

## Decision

koment ships an MCP server (`koment mcp`, stdio) exposing:

- `koment_get(file)` — annotations for a path, each with its resolution status
- `koment_search(query)` — full-text across annotation bodies

MCP is the delivery mechanism. The CLI (`show`, `check`, `list`) exists for
humans, CI and shell hooks.

## Consequences

- One implementation serves every MCP-capable agent. Adding a new client is
  configuration, not code.
- Resolution status travels with the annotation, so an agent can tell a
  confirmed annotation from a drifted one and weigh it accordingly. A drifted
  annotation must never be presented as if it were current.
- MCP tool schemas cost context in every request. Keep the surface to two tools;
  resist growing it. An unfiltered MCP server measured elsewhere in this stack
  cost ~154k tokens of schema per request — tool count is a real budget.
- stdio transport means no port, no auth, no network exposure.

## Alternatives rejected

- **Per-client hooks.** Highest fidelity — a `PreToolUse` hook can inject
  annotations without the agent asking. But it needs one implementation per
  client, and only some clients have a hook system. Revisit as an *addition*
  once MCP works, not as the foundation.
- **A generated `AGENTS.md` fragment.** Zero new machinery, works everywhere,
  but it is whole-repo context rather than the annotations for the file in hand,
  and it goes stale between regenerations.
- **An HTTP server.** Needs a port, lifecycle management and auth to read files
  the agent can already read. stdio has none of those problems.
  **Revisited in ADR 0011 and partly reversed:** "files the agent can already
  read" assumes the agent shares a filesystem with the checkout, which is untrue
  for containerised and hosted agents. HTTP is now offered alongside stdio; the
  port and lifecycle costs named here were correct and are paid only on demand.
- **A language server (LSP).** Right shape for editors, wrong shape for agents,
  and a much larger protocol surface to implement correctly.
