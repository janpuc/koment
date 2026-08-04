# Claude Code

## Configure

Install a released `koment` binary, enable the repository with `koment agents
install`, then choose the repository marketplace plugin or the generated files
below. The plugin adds the same writable MCP server plus strict session-start
guidance and a Stop hook that refuses completion while policy fails:

```text
/plugin marketplace add janpuc/koment
/plugin install koment@janpuc-tools
```

Install it at project scope. The completion hook expects a koment-enabled
repository and should not run in unrelated workspaces.

`koment agents install` writes this project configuration and the shared agent
contract. The resulting `.mcp.json` contains:

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

Commit it. Everyone who opens the repository gets the same declaration after
installing the released binary and accepting the repository trust boundary.

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
