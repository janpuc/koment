# VS Code

VS Code has native MCP support. Workspace servers go in `.vscode/mcp.json`.

## Configure

```json
{
  "servers": {
    "koment": {
      "command": "koment",
      "args": ["mcp"]
    }
  }
}
```

**The top-level key is `servers`, not `mcpServers`.** VS Code differs from
Claude Code and Cursor here, and a config copied from one of those will simply
be ignored.

`stdio` is the default for a server with a `command`, so it needs no `type`.
Remote servers are explicit:

```json
{
  "servers": {
    "koment": {
      "type": "http",
      "url": "http://127.0.0.1:8765"
    }
  }
}
```

started with `koment mcp --http 8765` from inside the repository.

## Verify

Open Chat, switch to **Agent** mode, and open the tools picker — `koment_get`
and `koment_search` should be listed. `MCP: List Servers` in the command palette
shows status and logs, which is where to look when a server won't start.

## Make it read them

Add to `.github/copilot-instructions.md`:

```markdown
Before editing any file, call `koment_get` on it and read the annotations.
Treat a `drifted` or `orphaned` annotation as history, not as current fact.
```

## Notes

- Commit `.vscode/mcp.json` and the whole team gets it. VS Code asks for trust
  before starting a workspace-defined server, which is worth knowing before you
  wonder why nothing happened.
