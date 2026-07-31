# Agent setup

koment speaks [MCP](https://modelcontextprotocol.io), so one server serves every
client. Pick yours:

| Client | Config file | Page |
|---|---|---|
| Claude Code | `.mcp.json` | [claude-code.md](claude-code.md) |
| Hermes Agent | `config.yaml` | [hermes.md](hermes.md) |
| OpenClaw | OpenClaw config (JSON5) | [openclaw.md](openclaw.md) |
| opencode | `opencode.json` | [opencode.md](opencode.md) |
| Codex CLI | `~/.codex/config.toml` | [codex.md](codex.md) |
| Cursor | `.cursor/mcp.json` | [cursor.md](cursor.md) |
| VS Code | `.vscode/mcp.json` | [vscode.md](vscode.md) |
| Zed | Zed `settings.json` | [zed.md](zed.md) |
| Anything else | — | [other.md](other.md) |

## What your agent gets

Two tools, deliberately. Every MCP tool costs context in every request, so the
surface stays small.

- **`koment_get(file)`** — annotations for a file, each with its resolution
  status. The one to call before editing something unfamiliar.
- **`koment_search(query)`** — full-text across annotation bodies, for when you
  know the topic but not the file.

Every annotation carries its status, and a `drifted` or `orphaned` one arrives
with an explicit warning that it describes code which has since changed. A stale
annotation is never presented as though it were current.

## The one gotcha, whichever client you use

**koment resolves your repository from the server's working directory.** It
walks up from there looking for `.koment/`, then `.git/`.

Most clients launch MCP servers with the working directory set to your open
workspace, which is what you want. If yours doesn't — or if you run the server
yourself over HTTP — start it from inside the repository, or the annotations it
serves will belong to a different project or none at all.

## Telling the agent to actually use it

Wiring up the server makes the tools *available*. It does not make an agent
*reach for them*. Add a line to whatever instruction file your client reads —
`AGENTS.md`, `CLAUDE.md`, `.cursorrules`, a system prompt:

```markdown
Before editing any file, call `koment_get` on it and read the annotations.
They hold reasoning that is deliberately not in the comments. An annotation
whose status is `drifted` or `orphaned` describes code that has since changed:
treat it as history, and say so rather than acting on it.
```

koment's own repository does exactly this — see [AGENTS.md](../../AGENTS.md).
