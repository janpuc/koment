# Claude Code

## Configure

`koment agents install` writes this project configuration and the shared agent
contract. The resulting `.mcp.json` contains:

Create `.mcp.json` in your repository root:

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

Commit it. Everyone who opens the repository — and every agent that runs in it —
gets the tools with no further setup.

Or add it from the command line:

```sh
claude mcp add koment -- koment mcp --write
```

## Verify

Restart Claude Code, then:

```
/mcp
```

`koment` should be listed with the three read tools and four write tools. Ask
it something concrete — *"what does koment say about
internal/store/ulid.go?"* — and check the answer matches `koment show
internal/store/ulid.go`.

## Make it use them

`koment agents install` maintains the managed block in `AGENTS.md`; the generated
`CLAUDE.md` imports it. Run `koment agents check` in CI so neither surface can
quietly drift.

## Notes

- The server is launched with your workspace as its working directory, which is
  what koment needs to find `.koment/`.
- `.mcp.json` is project-scoped. For a personal, all-projects setup use
  `claude mcp add --scope user koment -- koment mcp --write`, but note that the server
  then resolves the repository from wherever Claude Code happens to launch it.
