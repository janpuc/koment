# Codex CLI

Codex keeps MCP servers in TOML, at `~/.codex/config.toml` globally or
`.codex/config.toml` for a trusted project.

## Configure

```sh
codex mcp add koment -- koment mcp
```

That writes the entry for you. Or do it by hand:

```toml
[mcp_servers.koment]
command = "koment"
args = ["mcp"]
```

Project-scoped configuration lives at `.codex/config.toml` and applies only to
trusted projects — the more useful placement for koment, since annotations are
per-repository. Setting `cwd` pins the repository explicitly:

```toml
[mcp_servers.koment]
command = "koment"
args = ["mcp"]
cwd = "/path/to/your/repo"
```

No koment binary on `PATH`:

```toml
[mcp_servers.koment]
command = "go"
args = ["run", "github.com/janpuc/koment/cmd/koment@latest", "mcp"]
```

## Verify

```sh
codex mcp list
```

`koment` should appear. Then ask for something you can check against
`koment show <file>`.

## Make it read them

Codex reads `AGENTS.md`:

```markdown
Before editing any file, call `koment_get` on it and read the annotations.
Treat a `drifted` or `orphaned` annotation as history, not as current fact.
```

## Notes

- TOML table names are the server name: `[mcp_servers.koment]` produces a server
  called `koment`. Several repositories means several tables with distinct names
  and their own `cwd`.
- Codex also supports `env` (a nested `[mcp_servers.koment.env]` table) and
  `env_vars` for forwarding existing variables. koment needs neither — it reads
  local files and takes no configuration.
