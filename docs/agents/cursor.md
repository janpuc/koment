# Cursor

Cursor reads `.cursor/mcp.json` in the project, or `~/.cursor/mcp.json`
globally.

## Configure

Create `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "koment": {
      "command": "koment",
      "args": ["mcp"]
    }
  }
}
```

Commit it, and every contributor using Cursor gets the tools.

No koment binary on `PATH`:

```json
{
  "mcpServers": {
    "koment": {
      "command": "go",
      "args": ["run", "github.com/janpuc/koment/cmd/koment@latest", "mcp"]
    }
  }
}
```

## Verify

**Settings → MCP**. `koment` should show as connected with two tools. If it
shows an error, the usual cause is `koment` not being on the `PATH` Cursor
inherits — a GUI app launched from the Dock does not get your shell's `PATH`. An
absolute path in `command` settles it:

```json
{ "mcpServers": { "koment": { "command": "/opt/homebrew/bin/koment", "args": ["mcp"] } } }
```

## Make it read them

Add a rule in `.cursor/rules/` (or `.cursorrules`):

```markdown
Before editing any file, call `koment_get` on it and read the annotations.
They hold reasoning that is deliberately not in the comments. Treat a
`drifted` or `orphaned` annotation as history, not as current fact.
```

## Notes

- Project config wins over global, so a repository can pin its own setup.
- Cursor launches the server with the workspace as its working directory, which
  is how koment finds `.koment/`.
