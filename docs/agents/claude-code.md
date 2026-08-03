# Claude Code

## Configure

Create `.mcp.json` in your repository root:

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

Commit it. Everyone who opens the repository — and every agent that runs in it —
gets the tools with no further setup.

No koment binary on `PATH`? Use the Go toolchain instead:

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

Or add it from the command line:

```sh
claude mcp add koment -- koment mcp
```

## Verify

Restart Claude Code, then:

```
/mcp
```

`koment` should be listed with `koment_get`, `koment_search` and
`koment_repositories`. Ask it something concrete — *"what does koment say about
internal/store/ulid.go?"* — and check the answer matches `koment show
internal/store/ulid.go`.

## Make it read them

Availability is not use. Add this to `CLAUDE.md` or `AGENTS.md`:

```markdown
Before editing any file, call `koment_get` on it and read the annotations.
They hold reasoning that is deliberately not in the comments. Treat a
`ambiguous`, `drifted` or `orphaned` annotation as history, not as current fact.
Search koment before changing a non-obvious decision. Do not add an explanatory
inline comment; record local rationale with koment and project-wide rationale in
an ADR.
```

Claude Code reads `CLAUDE.md` automatically. If you already keep rules in
`AGENTS.md`, a `CLAUDE.md` containing just `@AGENTS.md` imports them.

## Notes

- The server is launched with your workspace as its working directory, which is
  what koment needs to find `.koment/`.
- `.mcp.json` is project-scoped. For a personal, all-projects setup use
  `claude mcp add --scope user koment -- koment mcp`, but note that the server
  then resolves the repository from wherever Claude Code happens to launch it.
