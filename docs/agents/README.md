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

Three tools, deliberately. Every MCP tool costs context in every request, so the
surface stays small.

- **`koment_get(file)`** — annotations for a file, each with its resolution
  status. The one to call before editing something unfamiliar.
- **`koment_search(query)`** — full-text across annotation bodies, for when you
  know the topic but not the file.
- **`koment_repositories()`** — the assigned repositories and their resolution
  counts, for an intentional cross-repository operation.

Every annotation carries its status. `ambiguous`, `drifted` and `orphaned`
records arrive with an explicit warning and must be treated as history. An
uncertain annotation is never presented as though it were current.

## The one gotcha, whichever client you use

**koment resolves your repository from the server's working directory.** It
walks up from there looking for `.koment/`, then `.git/`.

Most clients launch MCP servers with the working directory set to your open
workspace, which is what you want. If yours doesn't — or if you run the server
yourself over HTTP — start it from inside the repository, or the annotations it
serves will belong to a different project or none at all.

## Give the agent a strict contract

Wiring up the server makes the tools *available*. It does not make an agent
*reach for them*. Put this contract in the repository instruction surface your
client reads:

```markdown
Before editing an existing file, call `koment_get` for it. Search koment before
changing a non-obvious decision. Treat ambiguous, drifted and orphaned records
as history, never current fact. Do not add an explanatory inline comment:
rename, extract, introduce a named type or constant, restructure, then create a
koment annotation with honest agent authorship. Run `koment check` before
finishing, and do not report success while it fails.
```

koment's own repository does exactly this — see [AGENTS.md](../../AGENTS.md).

Repository instructions and client hooks are early guardrails, not a security
boundary: an agent client can decline trust, disable instructions or write
through an unhooked process. Today `koment check` enforces annotation resolution,
not the absence of explanatory comments. The target enforceable outcome is a
required `koment comments check` status on a protected branch, paired with
`koment check`. [ADR 0108](../decisions/0108-layer-agent-guidance-behind-an-authoritative-policy-gate.md)
defines the version-1 policy and generated adapters that will remove the manual
copy step once local agent writes and comment-intent conversion exist.
