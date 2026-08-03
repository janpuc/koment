# Cursor

Cursor reads `.cursor/mcp.json` in the project, or `~/.cursor/mcp.json`
globally.

## Configure

`koment agents install` writes this project configuration and the managed Cursor
rule. The resulting `.cursor/mcp.json` contains:

Create `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "koment": {
      "command": "koment",
      "args": ["mcp", "--write"]
    }
  }
}
```

Commit it, and every contributor using Cursor gets the tools.

## Verify

**Settings → MCP**. `koment` should show as connected with seven tools. If it
shows an error, the usual cause is `koment` not being on the `PATH` Cursor
inherits — a GUI app launched from the Dock does not get your shell's `PATH`. An
absolute path in `command` settles it:

```json
{ "mcpServers": { "koment": { "command": "/opt/homebrew/bin/koment", "args": ["mcp", "--write"] } } }
```

## Make it use them

The generated `.cursor/rules/koment.mdc` carries the strict contract. Run
`koment agents check` in CI so the rule and MCP configuration cannot quietly
drift.

## Notes

- Project config wins over global, so a repository can pin its own setup.
- Cursor launches the server with the workspace as its working directory, which
  is how koment finds `.koment/`.
