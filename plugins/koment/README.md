# koment plugins

This directory ships the agent marketplace plugins for koment. Each subdirectory
contains a self-contained integration for one agent runtime.

## Claude Code

Install a released `koment` binary on `PATH`, then enable koment in the target
repository:

```sh
koment agents install
```

Add this repository as a Claude marketplace and install the plugin at project
scope:

```text
/plugin marketplace add koment-dev/koment
/plugin install koment@koment-dev
```

The plugin is intentionally project-scoped: its Stop hook enforces a koment
repository and should not run in unrelated workspaces. Restart Claude Code or
run `/reload-plugins` after installation.

## OpenCode

Install a released `koment` binary on `PATH`, then bootstrap the repository with
the OpenCode adapter:

```sh
koment bootstrap --agents opencode --non-interactive
```

Or install the plugin directly from Git:

```json
{
  "plugin": ["koment-dev/koment/plugins/koment/.opencode-plugin"]
}
```

The plugin registers `tool.execute.before` to deny ordinary Go comment intent
and `dispose` to run the policy gate before the session completes. Restart
OpenCode after installation.

See [plugins/koment/.opencode-plugin/README.md](.opencode-plugin/README.md) for
detailed installation and verification instructions.

## Generated adapters

`koment agents install` regenerates the following files for each selected adapter:

- `.mcp.json` and `CLAUDE.md` (Claude)
- `.github/copilot-instructions.md` and `.vscode/mcp.json` (Copilot)
- `.cursor/rules/koment.mdc` and `.cursor/mcp.json` (Cursor)
- `.codex/hooks.json` and `.codex/config.toml` (Codex)
- `.opencode/plugins/koment.js` and `opencode.json` (OpenCode)
- `AGENTS.md` (managed contract shared by every client)

`koment agents check` flags drift on any of these surfaces.
