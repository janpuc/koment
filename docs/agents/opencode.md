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
      "command": ["koment", "mcp", "--write"]
    }
  },
  "plugin": ["./.opencode/plugins/koment.js"]
}
```

Note the shape: **`command` is a single array** holding the executable *and* its
arguments — not a `command` string plus a separate `args` list, which is what
most other clients use. Copying a config from elsewhere is the usual way to get
this wrong.

MCP servers are enabled by default; add `"enabled": false` to switch one off
without deleting it.

The `plugin` entry loads `.opencode/plugins/koment.js`, which mirrors
`.codex/hooks.json`: it denies ordinary Go comment intent on `edit`/`write`
and re-runs the policy gate at session idle. The plugin shells out to
`koment`, so `koment` must be on `PATH` (or set up via the project setup
Action). Skip the `plugin` entry if you only want the read tools.

## OpenCode plugin

If you prefer a plugin directory over a generated file, the same hooks ship in
`plugins/koment/.opencode-plugin/` and can be referenced by Git path:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "koment": {
      "type": "local",
      "command": ["koment", "mcp", "--write"]
    }
  },
  "plugin": ["koment-dev/koment/plugins/koment/.opencode-plugin"]
}
```

This path form is convenient when several repositories share the plugin from
one upstream reference. Restart OpenCode after changing either config so the
hook is reloaded.

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
- `koment agents install` regenerates both `opencode.json` and
  `.opencode/plugins/koment.js`; `koment agents check` flags drift on both.
- The decision to ship both a generated adapter and a plugin directory is
  recorded in ADR 0126.
