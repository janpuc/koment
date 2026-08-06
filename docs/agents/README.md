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

See also [allowed-annotations.md](allowed-annotations.md) for the shape and
common examples of `spec.comments.allowedAnnotations`, the user-configurable
extension to the intrinsic comment classes.

## Install the repository contract

```sh
koment agents install
```

This creates the strict `.koment/policy.yaml`, refreshes supported repository
instruction files and client hooks, and configures writable local MCP where the
client supports project configuration. Existing unrelated configuration is
preserved. Commit the generated files so the procedure starts with the
repository, not a person's workstation.

## What your agent gets

Every local server has three read tools:

- **`koment_get(file)`** — annotations for a file, each with its resolution
  status. The one to call before editing something unfamiliar.
- **`koment_search(query)`** — full-text across annotation bodies, for when you
  know the topic but not the file.
- **`koment_repositories()`** — the assigned repositories and their resolution
  counts, for an intentional cross-repository operation.

`koment mcp --write` over stdio adds four mutation tools:

- **`koment_add`** — create an annotation with the MCP client recorded as an agent.
- **`koment_reanchor`** — explicitly confirm a changed anchor.
- **`koment_convert_comment`** — record rationale before removing its inline comment.
- **`koment_acknowledge_comment`** — keep an exceptional comment only with an exact, attributable acknowledgement.

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

## What is actually enforced

Instructions tell a cooperative agent what to do. Supported pre-tool hooks can
deny obvious comment-adding patches, and stop hooks make an agent continue when
resolution, comment policy or generated adapters fail. Neither is a security
boundary: a client can decline trust or write through an unhooked process.

The authoritative boundary is CI. Require `koment check`, `koment comments
check` and `koment agents check` on the protected branch. Then an ordinary
comment or a weakened adapter cannot land regardless of which editor or agent
created it. [ADR 0108](../decisions/0108-layer-agent-guidance-behind-an-authoritative-policy-gate.md)
records the layers and their limits.
