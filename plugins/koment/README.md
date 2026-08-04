# koment for Claude Code

This plugin starts the writable koment MCP server, places the strict repository
procedure in session context, and prevents a turn from completing while an
annotation, inline-comment policy, or generated agent adapter is invalid.

Install a released `koment` binary on `PATH`, then enable koment in the target
repository:

```sh
koment agents install
```

Add this repository as a Claude marketplace and install the plugin at project
scope:

```text
/plugin marketplace add janpuc/koment
/plugin install koment@janpuc-tools
```

The plugin is intentionally project-scoped: its Stop hook enforces a koment
repository and should not run in unrelated workspaces. Restart Claude Code or
run `/reload-plugins` after installation.
