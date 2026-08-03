# Codex CLI

Codex keeps MCP servers in TOML, at `~/.codex/config.toml` globally or
`.codex/config.toml` for a trusted project.

## Configure

`koment agents install` writes the project MCP entry and supported Codex hooks.

```sh
codex mcp add koment -- koment mcp --write
```

That writes the entry for you. Or do it by hand:

```toml
[mcp_servers.koment]
command = "koment"
args = ["mcp", "--write"]
```

Project-scoped configuration lives at `.codex/config.toml` and applies only to
trusted projects — the more useful placement for koment, since annotations are
per-repository. Setting `cwd` pins the repository explicitly:

```toml
[mcp_servers.koment]
command = "koment"
args = ["mcp", "--write"]
cwd = "/path/to/your/repo"
```

## Verify

```sh
codex mcp list
```

`koment` should appear. Then ask for something you can check against
`koment show <file>`.

## Make it use them

Codex reads the managed contract in `AGENTS.md`. The generated pre-tool hook
denies ordinary Go comment intent in `apply_patch`; its stop hook checks
annotations, comments and adapters before the turn can finish. Run `koment
agents check` in CI because hooks remain a workstation guardrail, not the
authoritative boundary.

## Notes

- TOML table names are the server name: `[mcp_servers.koment]` produces a server
  called `koment`. Several repositories means several tables with distinct names
  and their own `cwd`.
- Codex also supports `env` (a nested `[mcp_servers.koment.env]` table) and
  `env_vars` for forwarding existing variables. koment needs neither — it reads
  local files and takes no configuration.
