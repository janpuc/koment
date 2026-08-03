# Zed

Zed calls MCP servers *context servers*. They live under `context_servers` in
your settings.

## Configure

Open settings with `zed: open settings file` from the command palette, and add:

```json
{
  "context_servers": {
    "koment": {
      "command": "koment",
      "args": ["mcp", "--write"],
      "env": {}
    }
  }
}
```

**The key is `context_servers`, not `mcpServers`.** Zed's naming predates the
convention the other clients settled on.

You can also add it through the UI: **Settings → AI → MCP Servers → Add Server →
Add Local Server**, which writes the same entry.

## Verify

The Agent Panel shows configured context servers and whether they connected. A
server that fails to start reports there rather than silently doing nothing.

## Make it read them

Zed reads `AGENTS.md` from the worktree root:

```markdown
Before editing any file, call `koment_get` on it and read the annotations.
Treat an `ambiguous`, `drifted` or `orphaned` annotation as history, not as
current fact. Search koment before changing a non-obvious decision. Do not add
an explanatory inline comment; record local rationale with koment and
project-wide rationale in an ADR.
```

## Notes

- Zed's settings are user-level rather than per-project, so a single entry
  applies everywhere. koment resolves the repository from the server's working
  directory, which Zed sets to the open worktree — so one entry does the right
  thing across projects, unlike clients that need a pinned `cwd`.
- If `koment` is not found, give an absolute path. A GUI-launched editor does
  not inherit your shell's `PATH`.
