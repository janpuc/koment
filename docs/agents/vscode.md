# VS Code

VS Code has native MCP support. Workspace servers go in `.vscode/mcp.json`.
The koment extension is complementary: MCP gives Chat agents tools, while the
extension runs `koment lsp` for inline human views, diagnostics and native
mutation actions.

## Configure

Install the signed VSIX from a koment release, or use the VS Code Marketplace
or Open VSX after the publisher channel is active. The extension requires the
released `koment` binary on `PATH`; `koment.binaryPath` selects another location.

`koment agents install` writes this project configuration and the matching
Copilot instructions. The resulting `.vscode/mcp.json` contains:

```json
{
  "servers": {
    "koment": {
      "command": "koment",
      "args": ["mcp", "--write"]
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

Open Chat, switch to **Agent** mode, and open the tools picker — `koment_get`,
`koment_search`, `koment_repositories` and the four mutation tools should be
listed. `MCP: List Servers`
in the command palette shows status and logs, which is where to look when a
server won't start.

## Make it use them

The generated `.github/copilot-instructions.md` carries the strict contract.
Run `koment agents check` in CI so the instructions and MCP configuration cannot
quietly drift.

## Notes

- Commit `.vscode/mcp.json` and the whole team gets it. VS Code asks for trust
  before starting a workspace-defined server, which is worth knowing before you
  wonder why nothing happened.
