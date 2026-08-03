# opencode

[opencode](https://opencode.ai) reads `opencode.json` (or `opencode.jsonc`) from
your project root.

## Configure

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "koment": {
      "type": "local",
      "command": ["koment", "mcp"]
    }
  }
}
```

Note the shape: **`command` is a single array** holding the executable *and* its
arguments — not a `command` string plus a separate `args` list, which is what
most other clients use. Copying a config from elsewhere is the usual way to get
this wrong.

MCP servers are enabled by default; add `"enabled": false` to switch one off
without deleting it.

No koment binary on `PATH`:

```json
{
  "mcp": {
    "koment": {
      "type": "local",
      "command": ["go", "run", "github.com/janpuc/koment/cmd/koment@latest", "mcp"]
    }
  }
}
```

## Remote

```sh
cd /path/to/your/repo && koment mcp --http 8765
```

```json
{
  "mcp": {
    "koment": {
      "type": "remote",
      "url": "http://127.0.0.1:8765"
    }
  }
}
```

## Make it read them

opencode reads `AGENTS.md`:

```markdown
Before editing any file, call `koment_get` on it and read the annotations.
Treat an `ambiguous`, `drifted` or `orphaned` annotation as history, not as
current fact. Search koment before changing a non-obvious decision. Do not add
an explanatory inline comment; record local rationale with koment and
project-wide rationale in an ADR.
```

## Notes

- Commit `opencode.json` and every contributor gets the tools automatically.
